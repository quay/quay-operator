/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	quayredhatcomv1 "github.com/quay/quay-operator/apis/quay/v1"
	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const (
	upgradePollInterval  = time.Second * 10
	upgradePollTimeout   = time.Second * 6000
	creationPollInterval = time.Second * 1
	creationPollTimeout  = time.Second * 600

	GrafanaDashboardConfigMapNameSuffix = "grafana-dashboard-quay"
	GrafanaTitleJSONPath                = "title"
	GrafanaNamespaceFilterJSONPath      = "templating.list.1.options.0.value"
	GrafanaServiceFilterJSONPath        = "templating.list.2.options.0.value"
	ClusterMonitoringLabelKey           = "openshift.io/cluster-monitoring"
	QuayDashboardJSONKey                = "quay.json"
	QuayOperatorManagedLabelKey         = "quay-operator/managed-label"
	QuayOperatorFinalizer               = "quay-operator/finalizer"
)

// QuayRegistryReconciler reconciles a QuayRegistry object
type QuayRegistryReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	EventRecorder  record.EventRecorder
	WatchNamespace string
	Mtx            *sync.Mutex
}

// manageQuayDeletion makes sure that we process a QuayRegistry once it is flagged for
// deletion. Removes the finalizer once it is done, requeues an event for the registry
// in case of failure.
func (r *QuayRegistryReconciler) manageQuayDeletion(
	ctx context.Context, quay *v1.QuayRegistry, log logr.Logger,
) (ctrl.Result, error) {
	log.Info("`QuayRegistry` to be deleted")
	if !controllerutil.ContainsFinalizer(quay, QuayOperatorFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.finalizeQuay(ctx, quay); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	controllerutil.RemoveFinalizer(quay, QuayOperatorFinalizer)
	if err := r.Update(ctx, quay); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// checkMigrationStatus checks the migration job status for a given QuayRegistry instance.
// This function verifies if the job ran successfully and sets its condition properly. The
// result of this function is always a ctrl.Result with a proper reschedule value. Once
// migration job has been finished this function sets quay.Status.CurrentVersion to the
// value of v1.QuayVersionCurrent, indicating that we have migrated to the current version.
func (r *QuayRegistryReconciler) checkMigrationStatus(
	ctx context.Context, quay *v1.QuayRegistry, log logr.Logger,
) (ctrl.Result, error) {
	log.Info("checking Quay upgrade `Job` completion")

	nsn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", quay.GetName(), v1.QuayUpgradeJobName),
		Namespace: quay.GetNamespace(),
	}

	var job batchv1.Job
	if err := r.Client.Get(ctx, nsn, &job); err != nil {
		// similarly to when a v1.ConditionReasonMigrationsFailed occurs,
		// when the upgrade job is expected to exist but doesn't
		// (i.e. someone manually removed it) we want the reconcile loop
		// to run in its entirety, so we change the condition reason to
		// something other than migrations in progress.
		if errors.IsNotFound(err) {
			if err = r.updateWithCondition(
				ctx,
				quay,
				v1.ConditionComponentsCreated,
				metav1.ConditionFalse,
				v1.ConditionReasonMigrationsJobMissing,
				"upgrade job not found",
			); err != nil {
				log.Error(err, "failed to update `conditions` of `QuayRegistry`")
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		log.Error(err, "could not retrieve Quay upgrade `Job`")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if job.Status.Active == 1 {
		log.Info("Upgrade job running, requeueing reconcile...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if job.Status.Succeeded == 1 {
		log.Info("Quay upgrade complete, updating `status.currentVersion`")

		condition := v1.Condition{
			Type:               v1.ConditionComponentsCreated,
			Status:             metav1.ConditionTrue,
			Reason:             v1.ConditionReasonComponentsCreationSuccess,
			Message:            "All registry components created",
			LastUpdateTime:     metav1.Now(),
			LastTransitionTime: metav1.Now(),
		}
		quay.Status.CurrentVersion = v1.QuayVersionCurrent
		quay.Status.Conditions = v1.SetCondition(quay.Status.Conditions, condition)

		if err := r.Client.Status().Update(ctx, quay); err != nil {
			log.Error(err, "could not update status with current version")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		log.Info("successfully updated `status` after Quay upgrade")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log.Info("upgrade job failed or crashed.", "upgrade-job-status", job.Status)

	// a kube job can be Active, Succeeded or Failed.
	// we explicitly check for Active and Succeeded above. if no pods
	// are in either state it means the job either failed or crashed.
	// crashed pods are not marked as Failed, so we don't check.
	//
	// when the job crashes or fails, the next reconciliation to run
	// will think that migrations are currently not running, since
	// we change the condition reason to v1.ConditionReasonMigrationsFailed.
	// this doesn't necessarily describe reality, because kube will
	// retry failed jobs for us, but it's desired behaviour because
	// when the migration job fails due to misconfiguration, then the
	// reconcile function should be allowed to proceed.
	msg := "failed to run migrations"
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			msg = cond.Message
			break
		}
	}

	if err := r.updateWithCondition(
		ctx,
		quay,
		v1.ConditionComponentsCreated,
		metav1.ConditionFalse,
		v1.ConditionReasonMigrationsFailed,
		msg,
	); err != nil {
		log.Error(err, "failed to update `conditions` of `QuayRegistry`")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// createInitialBundleSecret creates a new config bundle secret for provided QuayRegistry
// object, the created bundle contains a default quay config and is then populated as
// ConfigBundleSecret. QuayRegistry is updated and a reschedule Result is returned.
func (r *QuayRegistryReconciler) createInitialBundleSecret(
	ctx context.Context, quay *v1.QuayRegistry, log logr.Logger,
) (ctrl.Result, error) {
	log.Info("`spec.configBundleSecret` is unset. Creating base `Secret`")

	baseConfigBundle := v1.EnsureOwnerReference(
		quay,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("%s-config-bundle-", quay.GetName()),
				Namespace:    quay.GetNamespace(),
			},
			Data: map[string][]byte{
				"config.yaml": encode(kustomize.BaseConfig()),
			},
		},
	)

	if err := r.Client.Create(ctx, baseConfigBundle); err != nil {
		return r.reconcileWithCondition(
			ctx,
			quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("unable to create base config bundle `Secret`: %s", err),
		)
	}

	quay.Spec.ConfigBundleSecret = baseConfigBundle.GetName()
	if err := r.Client.Update(ctx, quay); err != nil {
		log.Error(err, "unable to update `spec.configBundleSecret`")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	log.Info("successfully updated `spec.configBundleSecret`")
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// GetConfigBundleSecret returns the secret used to configure provided QuayRegistry
// instance.
func (r *QuayRegistryReconciler) GetConfigBundleSecret(
	ctx context.Context, quay *v1.QuayRegistry,
) (*corev1.Secret, error) {
	secnsn := types.NamespacedName{
		Namespace: quay.GetNamespace(),
		Name:      quay.Spec.ConfigBundleSecret,
	}

	bundle := &corev1.Secret{}
	if err := r.Get(ctx, secnsn, bundle); err != nil {
		return nil, err
	}

	return bundle, nil
}

// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries/status,verbs=get;update;patch

func (r *QuayRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Mtx.Lock()
	defer r.Mtx.Unlock()

	log := r.Log.WithValues("quayregistry", req.NamespacedName)
	log.Info("begin reconcile")

	var quay v1.QuayRegistry
	if err := r.Client.Get(ctx, req.NamespacedName, &quay); err != nil {
		if errors.IsNotFound(err) {
			log.Info("`QuayRegistry` deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to retrieve QuayRegistry")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	updatedQuay := quay.DeepCopy()
	if v1.FlaggedForDeletion(updatedQuay) {
		return r.manageQuayDeletion(ctx, updatedQuay, log)
	}

	if v1.MigrationsRunning(updatedQuay) {
		return r.checkMigrationStatus(ctx, updatedQuay, log)
	}

	if v1.NeedsBundleSecret(updatedQuay) {
		return r.createInitialBundleSecret(ctx, updatedQuay, log)
	}

	configBundle, err := r.GetConfigBundleSecret(ctx, updatedQuay)
	if err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("unable to get `configBundleSecret`: %s", err),
		)
	}

	quayContext := quaycontext.NewQuayRegistryContext()
	r.checkManagedTLS(quayContext, configBundle)

	if err := r.checkManagedKeys(ctx, quayContext, updatedQuay); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("unable to retrieve managed keys `Secret`: %s", err),
		)
	}

	if err := r.checkRoutesAvailable(
		ctx, quayContext, updatedQuay, configBundle,
	); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonRouteComponentDependencyError,
			fmt.Sprintf("could not check for `Routes` API: %s", err),
		)
	}

	osmanaged := v1.ComponentIsManaged(updatedQuay.Spec.Components, v1.ComponentObjectStorage)
	if err := r.checkObjectBucketClaimsAvailable(
		ctx, quayContext, updatedQuay,
	); err != nil && osmanaged {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonObjectStorageComponentDependencyError,
			fmt.Sprintf("error checking for object storage support: %s", err),
		)
	}

	if err := r.checkBuildManagerAvailable(quayContext, configBundle); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("could not check for build manager support: %s", err),
		)
	}

	if v1.ComponentIsManaged(updatedQuay.Spec.Components, v1.ComponentMonitoring) {
		if err := r.checkMonitoringAvailable(ctx, quayContext); err != nil {
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonMonitoringComponentDependencyError,
				fmt.Sprintf("could not check for monitoring support: %s", err),
			)
		}
	}

	updatedQuay, err = v1.EnsureDefaultComponents(quayContext, updatedQuay)
	if err != nil {
		log.Error(err, "could not ensure default `spec.components`")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	if err := v1.ValidateOverrides(updatedQuay); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonComponentOverrideInvalid,
			fmt.Sprintf("could not validate overrides on spec.components: %s", err),
		)
	}

	if !v1.ComponentsMatch(quay.Spec.Components, updatedQuay.Spec.Components) {
		log.Info("updating QuayRegistry `spec.components` to include defaults")
		if err = r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `spec.components` to include defaults")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	var usercfg map[string]interface{}
	if err = yaml.Unmarshal(configBundle.Data["config.yaml"], &usercfg); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			err.Error(),
		)
	}

	updatedQuay.Status.Conditions = v1.RemoveCondition(
		updatedQuay.Status.Conditions, v1.ConditionTypeRolloutBlocked,
	)

	usercerts := map[string][]byte{}
	if _, ok := configBundle.Data["ssl.cert"]; ok {
		usercerts["ssl.cert"] = configBundle.Data["ssl.cert"]
	}
	if _, ok := configBundle.Data["ssl.key"]; ok {
		usercerts["ssl.key"] = configBundle.Data["ssl.key"]
	}

	for _, cmp := range updatedQuay.Spec.Components {
		contains, err := kustomize.ContainsComponentConfig(usercfg, usercerts, cmp)
		if err != nil {
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				err.Error(),
			)
		}

		if cmp.Managed && contains && !v1.ComponentSupportsConfigWhenManaged(cmp) {
			// if the component is marked as managed but the user has provided
			// config for it (in config bundle secret) then we have a problem
			// there are a few components that don't care if the config has
			// been provided by the user or not, these are evaluated in the
			// function ComponentSupportsConfigWhenManaged().
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				fmt.Sprintf(
					"%s component marked as managed, but "+
						"`configBundleSecret` contains required fields",
					cmp.Kind,
				),
			)
		} else if !cmp.Managed && !contains && v1.RequiredComponent(cmp.Kind) {
			// here we have a component that is not managed, is required and the
			// user has not provided a config for it (in config bundle secret).
			// we can't proceed.
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				fmt.Sprintf(
					"required component `%s` marked as unmanaged, but "+
						"`configBundleSecret` is missing necessary fields",
					cmp.Kind,
				),
			)
		}
	}

	log.Info("inflating QuayRegistry into Kubernetes objects")
	deploymentObjects, err := kustomize.Inflate(quayContext, updatedQuay, configBundle, log)
	if err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonComponentCreationFailed,
			fmt.Sprintf("could not inflate kubernetes objects: %s", err),
		)
	}

	for _, obj := range kustomize.EnsureCreationOrder(deploymentObjects) {
		// For metrics and dashboards to work, we need to deploy the Grafana ConfigMap
		// in the `openshift-config-managed` namespace and add the label
		// `openshift.io/cluster-monitoring: true` to the registry namespace
		if quayContext.SupportsMonitoring && isGrafanaConfigMap(obj) {
			obj.SetNamespace(GrafanaDashboardConfigNamespace)
			if obj, err = updateGrafanaDashboardData(
				obj, updatedQuay.GetName(), updatedQuay.GetNamespace(),
			); err != nil {
				return r.reconcileWithCondition(
					ctx,
					&quay,
					v1.ConditionTypeRolloutBlocked,
					metav1.ConditionTrue,
					v1.ConditionReasonMonitoringComponentDependencyError,
					fmt.Sprintf("unable to update title on Grafana %s", err),
				)
			}
		}

		if err := r.createOrUpdateObject(ctx, obj, quay); err != nil {
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonComponentCreationFailed,
				fmt.Sprintf("error creating object: %s", err),
			)
		}
	}

	if quayContext.SupportsMonitoring {
		if err := r.patchNamespaceForMonitoring(ctx, quay); err != nil {
			return r.reconcileWithCondition(
				ctx,
				updatedQuay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonMonitoringComponentDependencyError,
				err.Error(),
			)
		}
	}

	updatedQuay, _ = v1.EnsureConfigEditorEndpoint(quayContext, updatedQuay)
	cfgsecret := configEditorCredentialsSecretFrom(deploymentObjects)
	updatedQuay.Status.ConfigEditorCredentialsSecret = cfgsecret

	rolloutBlocked := v1.GetCondition(
		updatedQuay.Status.Conditions,
		v1.ConditionTypeRolloutBlocked,
	)
	if rolloutBlocked != nil {
		invalid := rolloutBlocked.Reason == v1.ConditionReasonConfigInvalid
		if invalid && rolloutBlocked.Status == metav1.ConditionTrue {
			return r.reconcileWithCondition(
				ctx,
				updatedQuay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				rolloutBlocked.Message,
			)
		}
	}

	if err := r.updateWithCondition(
		ctx,
		updatedQuay,
		v1.ConditionTypeRolloutBlocked,
		metav1.ConditionFalse,
		v1.ConditionReasonComponentsCreationSuccess,
		"All objects created/updated successfully",
	); err != nil {
		log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	osmanaged = v1.ComponentIsManaged(updatedQuay.Spec.Components, "objectstorage")
	if osmanaged && !quayContext.ObjectStorageInitialized {
		r.Log.Info("requeuing to populate values for managed component: `objectstorage`")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	upToDate := v1.EnsureRegistryEndpoint(quayContext, updatedQuay, usercfg)
	if !upToDate {
		if err = r.Client.Status().Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `registryEndpoint` of `QuayRegistry`")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// if the version differ then it means that the operator was upgraded and we need
	// to wait until the database upgrade job finishes. sets a condition here and
	// returns.
	if updatedQuay.Status.CurrentVersion != v1.QuayVersionCurrent {
		if err := r.updateWithCondition(
			ctx,
			updatedQuay,
			v1.ConditionComponentsCreated,
			metav1.ConditionFalse,
			v1.ConditionReasonMigrationsInProgress,
			"running database migrations",
		); err != nil {
			log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if !controllerutil.ContainsFinalizer(updatedQuay, QuayOperatorFinalizer) {
		controllerutil.AddFinalizer(updatedQuay, QuayOperatorFinalizer)
		if err := r.Update(ctx, updatedQuay); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// updateGrafanaDashboardData parses the Grafana Dashboard ConfigMap and updates the title and labels to filter the query by
func updateGrafanaDashboardData(obj client.Object, quayName string, quayNamespace string) (client.Object, error) {
	updatedObj := obj.DeepCopyObject()
	configMapObj := updatedObj.(*corev1.ConfigMap)

	dashboardConfigJSON := configMapObj.Data[QuayDashboardJSONKey]

	newTitle := fmt.Sprintf("Quay - %s - %s", quayNamespace, quayName)
	dashboardConfigJSON, err := sjson.Set(dashboardConfigJSON, GrafanaTitleJSONPath, newTitle)
	if err != nil {
		return nil, err
	}

	dashboardConfigJSON, err = sjson.Set(dashboardConfigJSON, GrafanaNamespaceFilterJSONPath, quayNamespace)
	if err != nil {
		return nil, err
	}

	metricsServiceName := fmt.Sprintf("%s-quay-metrics", quayName)
	dashboardConfigJSON, err = sjson.Set(dashboardConfigJSON, GrafanaServiceFilterJSONPath, metricsServiceName)
	if err != nil {
		return nil, err
	}

	configMapObj.Data[QuayDashboardJSONKey] = dashboardConfigJSON
	return configMapObj, nil
}

// isGrafanaConfigMap checks if an Object is the Grafana ConfigMap used in the monitoring
// component.
func isGrafanaConfigMap(obj client.Object) bool {
	if !strings.HasSuffix(obj.GetName(), GrafanaDashboardConfigMapNameSuffix) {
		return false
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Version == "v1" && gvk.Kind == "ConfigMap"
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}

func decode(bytes []byte) interface{} {
	var value interface{}
	_ = yaml.Unmarshal(bytes, &value)

	return value
}

func (r *QuayRegistryReconciler) createOrUpdateObject(
	ctx context.Context, obj client.Object, quay v1.QuayRegistry,
) error {
	groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()

	immutableResources := map[string]bool{
		schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}.String(): true,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}.String():  true,
	}

	log := r.Log.WithValues(
		"quayregistry",
		quay.GetNamespace(),
		"Name",
		obj.GetName(),
		"GroupVersionKind",
		groupVersionKind,
	)

	log.Info("creating/updating object")

	// we set the owner in the object except when it belongs to a different namespace,
	// on this case we have only the grafana dashboard that lives in another place.
	obj = v1.EnsureOwnerReference(&quay, obj)
	if isGrafanaConfigMap(obj) {
		var err error
		if obj, err = v1.RemoveOwnerReference(&quay, obj); err != nil {
			log.Error(err, "could not remove `ownerReferences` from grafana config")
			return err
		}
	}

	// managedFields cannot be set on a PATCH.
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{})

	if immutableResources[groupVersionKind] {
		propagationPolicy := metav1.DeletePropagationForeground
		opts := &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}
		err := r.Client.Delete(ctx, obj, opts)
		if err != nil && !errors.IsNotFound(err) && !errors.IsAlreadyExists(err) {
			log.Error(err, "failed to delete immutable resource")
			return err
		}

		if err := wait.Poll(
			creationPollInterval,
			creationPollTimeout,
			func() (bool, error) {
				if err := r.Client.Create(ctx, obj); err == nil {
					return true, nil
				} else if errors.IsAlreadyExists(err) {
					return false, nil
				} else {
					return false, err
				}
			},
		); err != nil {
			log.Error(err, "failed to create immutable resource")
			return err
		}

		log.Info("succefully (re)created immutable resource")
	} else {
		opts := []client.PatchOption{
			client.ForceOwnership,
			client.FieldOwner("quay-operator"),
		}
		if err := r.Client.Patch(ctx, obj, client.Apply, opts...); err != nil {
			log.Error(err, "failed to create/update object")
			return err
		}
	}

	log.Info("finished creating/updating object")
	return nil
}

func (r *QuayRegistryReconciler) updateWithCondition(
	ctx context.Context,
	quay *v1.QuayRegistry,
	ctype v1.ConditionType,
	cstatus metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) error {
	condition := v1.Condition{
		Type:               ctype,
		Status:             cstatus,
		Reason:             reason,
		Message:            msg,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	quay.Status.Conditions = v1.SetCondition(quay.Status.Conditions, condition)
	quay.Status.LastUpdate = time.Now().UTC().String()
	return r.Client.Status().Update(ctx, quay)
}

// reconcileWithCondition sets the given condition on the `QuayRegistry` and returns a reconcile
// result rescheduling the next loop.
func (r *QuayRegistryReconciler) reconcileWithCondition(
	ctx context.Context,
	quay *v1.QuayRegistry,
	ctype v1.ConditionType,
	cstatus metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) (ctrl.Result, error) {
	err := r.updateWithCondition(ctx, quay, ctype, cstatus, reason, msg)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, err
}

// SetupWithManager initializes the controller manager
func (r *QuayRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// FIXME: Can we do this in the `init()` function in `main.go`...?
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add OpenShift `Route` API to scheme")

		return err
	}
	// FIXME: Can we do this in the `init()` function in `main.go`...?
	if err := objectbucket.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add `ObjectBucketClaim` API to scheme")

		return err
	}

	if err := prometheusv1.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add `PrometheusRule` API to scheme")

		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quayredhatcomv1.QuayRegistry{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		// TODO: Add `.Owns()` for every resource type we manage...
		Complete(r)
}

// patchNamespaceForMonitoring adds a few labels to the namespace, these labels are
// required to enable monitoring to "observer" the given namespace.
func (r *QuayRegistryReconciler) patchNamespaceForMonitoring(
	ctx context.Context, quay v1.QuayRegistry,
) error {
	nsn := types.NamespacedName{
		Name: quay.GetNamespace(),
	}

	var ns corev1.Namespace
	if err := r.Client.Get(ctx, nsn, &ns); err != nil {
		return err
	}

	updatedNs := ns.DeepCopy()
	labels := make(map[string]string)
	for k, v := range updatedNs.Labels {
		labels[k] = v
	}

	if val := labels[ClusterMonitoringLabelKey]; val == "true" {
		return nil
	}

	labels[ClusterMonitoringLabelKey] = "true"
	labels[QuayOperatorManagedLabelKey] = "true"
	updatedNs.Labels = labels

	patch := client.MergeFrom(&ns)
	return r.Client.Patch(ctx, updatedNs, patch)
}

func (r *QuayRegistryReconciler) cleanupNamespaceLabels(ctx context.Context, quay *v1.QuayRegistry) error {
	var ns corev1.Namespace
	err := r.Client.Get(ctx, types.NamespacedName{Name: quay.GetNamespace()}, &ns)

	if err != nil {
		return err
	}

	var quayRegistryList v1.QuayRegistryList
	listOps := client.ListOptions{
		Namespace: quay.GetNamespace(),
	}

	if err := r.Client.List(ctx, &quayRegistryList, &listOps); err != nil {
		return err
	}

	if ns.Labels != nil && ns.Labels[QuayOperatorManagedLabelKey] != "" && len(quayRegistryList.Items) == 1 {
		updatedNs := ns.DeepCopy()
		labels := make(map[string]string)
		for k, v := range updatedNs.Labels {
			labels[k] = v
		}
		delete(labels, ClusterMonitoringLabelKey)
		delete(labels, QuayOperatorManagedLabelKey)
		updatedNs.Labels = labels

		patch := client.MergeFrom(&ns)
		err = r.Client.Patch(context.Background(), updatedNs, patch)
		return err
	}

	return nil
}

func (r *QuayRegistryReconciler) cleanupGrafanaConfigMap(ctx context.Context, quay *v1.QuayRegistry) error {
	var grafanaConfigMap corev1.ConfigMap
	grafanaConfigMapName := types.NamespacedName{
		Name:      quay.GetName() + "-" + GrafanaDashboardConfigMapNameSuffix,
		Namespace: GrafanaDashboardConfigNamespace}

	if err := r.Client.Get(ctx, grafanaConfigMapName, &grafanaConfigMap); err == nil || !errors.IsNotFound(err) {
		return r.Client.Delete(ctx, &grafanaConfigMap)
	}

	return nil
}

func (r *QuayRegistryReconciler) finalizeQuay(ctx context.Context, quay *v1.QuayRegistry) error {
	// NOTE: `controller-runtime` hangs rather than return "forbidden" error if insufficient RBAC permissions, so we use `WatchNamespace` to skip (https://github.com/kubernetes-sigs/controller-runtime/issues/550).
	if r.WatchNamespace != "" {
		r.Log.Info("not running in all-namespaces mode, skipping finalizer step: namespace label cleanup")
	} else {
		r.Log.Info("cleaning up namespace labels")

		if err := r.cleanupNamespaceLabels(ctx, quay); err != nil {
			return err
		}
		r.Log.Info("successfully cleaned up namespace labels")
	}

	// NOTE: `controller-runtime` hangs rather than return "forbidden" error if insufficient RBAC permissions, so we use `WatchNamespace` to skip (https://github.com/kubernetes-sigs/controller-runtime/issues/550).
	if r.WatchNamespace != "" {
		r.Log.Info("not running in all-namespaces mode, skipping finalizer step: Grafana `ConfigMap` cleanup")
	} else {
		r.Log.Info("cleaning up Grafana `ConfigMap`")
		if err := r.cleanupGrafanaConfigMap(ctx, quay); err != nil {
			return err
		}
		r.Log.Info("successfully cleaned up grafana config map")
	}

	return nil
}
