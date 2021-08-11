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

	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v2"

	quayredhatcomv1 "github.com/quay/quay-operator/apis/quay/v1"
	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

// Some contants we use across this operator. XXX Do we need to export them?
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

// QuayRegistryReconciler reconciles a QuayRegistry object.
type QuayRegistryReconciler struct {
	client.Client

	Log            logr.Logger
	Scheme         *runtime.Scheme
	EventRecorder  record.EventRecorder
	WatchNamespace string
	Mtx            *sync.Mutex
}

// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries/status,verbs=get;update;patch

// handleQuayRegistryDeletion makes sure we remove any other object related to a QuayRegistry if
// it contains the finalizer.
func (r *QuayRegistryReconciler) handleQuayRegistryDeletion(
	ctx context.Context, quay *v1.QuayRegistry,
) error {
	if !controllerutil.ContainsFinalizer(quay, QuayOperatorFinalizer) {
		return nil
	}

	if err := r.finalizeQuay(ctx, quay); err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(quay, QuayOperatorFinalizer)
	return r.Update(ctx, quay)
}

// createConfigBundle creates a new config bundle for the provided QuayRegistry. Bundle contains
// what is considered to be the BaseConfig (according to kustomize.BaseConfig). This function may
// leave a dangling Secret not in use, see comments below.
func (r *QuayRegistryReconciler) createConfigBundle(
	ctx context.Context, quay *v1.QuayRegistry,
) error {
	bundle := v1.EnsureOwnerReference(
		quay,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: quay.GetName() + "-config-bundle-",
				Namespace:    quay.GetNamespace(),
			},
			Data: map[string][]byte{
				"config.yaml": encode(kustomize.BaseConfig()),
			},
		},
	)

	if err := r.Client.Create(ctx, bundle); err != nil {
		return fmt.Errorf("unable to create config bundle: %w", err)
	}

	quay.Spec.ConfigBundleSecret = bundle.GetName()
	if err := r.Client.Update(ctx, quay); err != nil {
		// XXX this failure will block the QuayRegistry deployment so no need to delete
		// the now dangling Secret we just created above. This needs to be refactored so
		// on the next Reconcile cycle we attempt again.
		return fmt.Errorf("error updating quay after bundle creation: %w", err)
	}

	return nil
}

// loadQuayRegistryContext verifies if necessary features work and populate provided
// QuayRegistryContext while doing so. It checks if database credentials exist, if TLS certs
// have been provided, if the cluster supports Routes and ObjectBucketClaims. Returns a Duration
// in case the error can be "retried" or an error.
func (r *QuayRegistryReconciler) loadQuayRegistryContext(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	bundle corev1.Secret,
) (time.Duration, error) {
	var err error
	var zero time.Duration

	if qctx, quay, err = r.checkManagedKeys(
		ctx, qctx, quay, bundle.Data,
	); err != nil {
		return zero, fmt.Errorf("unable to retrieve managed keys `Secret`: %w", err)
	}

	if qctx, quay, err = r.checkManagedTLS(
		ctx, qctx, quay, bundle.Data,
	); err != nil {
		return zero, fmt.Errorf("unable to retrieve managed TLS `Secret`: %w", err)
	}

	managedRT := v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentRoute)
	if qctx, quay, err = r.checkRoutesAvailable(
		ctx, qctx, quay, bundle.Data,
	); err != nil && managedRT {
		return zero, fmt.Errorf("could not check for `Routes` API: %w", err)
	}

	managedOS := v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentObjectStorage)
	if qctx, quay, err = r.checkObjectBucketClaimsAvailable(
		ctx, qctx, quay, bundle.Data,
	); err != nil && managedOS {
		return time.Second, fmt.Errorf("could not check `ObjectBucketClaims` API: %w", err)
	}

	if qctx, quay, err = r.checkBuildManagerAvailable(
		ctx, qctx, quay, bundle.Data,
	); err != nil {
		return zero, fmt.Errorf("could not check for build manager support: %w", err)
	}

	managedMN := v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentMonitoring)
	if qctx, quay, err = r.checkMonitoringAvailable(
		ctx, qctx, quay, bundle.Data,
	); err != nil && managedMN {
		return zero, fmt.Errorf("could not check for monitoring support: %w", err)
	}

	return zero, nil
}

// validateComponentsConfig checks if we have configuration for all mandatory components. This
// function also checks if user has provided configuration for a component that is managed, that
// is not an expected condition as well.
func (r *QuayRegistryReconciler) validateComponentsConfig(
	quay *v1.QuayRegistry, bundle *corev1.Secret,
) error {
	var rawcfg = bundle.Data["config.yaml"]
	var config map[string]interface{}
	if err := yaml.Unmarshal(rawcfg, &config); err != nil {
		return fmt.Errorf("unable to parse config bundle: %w", err)
	}

	for _, component := range quay.Spec.Components {
		contains, err := kustomize.ContainsComponentConfig(config, component)
		if err != nil {
			return fmt.Errorf("failed to update conditions of QuayRegistry: %w", err)
		}

		if component.Managed && contains && component.Kind != v1.ComponentRoute {
			return fmt.Errorf(
				"%s component marked as managed, but `configBundleSecret` "+
					"contains configuration fields",
				component.Kind,
			)
		} else if !component.Managed && v1.RequiredComponent(component.Kind) && !contains {
			return fmt.Errorf(
				"required component `%s` marked as unmanaged, but "+
					"`configBundleSecret` is missing necessary fields",
				component.Kind,
			)
		}
	}

	return nil
}

// verifyMigrationJob checks if database migration job has been finished. Once the migration is
// ended moves the status to Available.
func (r *QuayRegistryReconciler) verifyMigrationJob(
	ctx context.Context, quay *v1.QuayRegistry,
) (ctrl.Result, error) {
	log := r.Log.WithValues(
		"quayregistry",
		types.NamespacedName{
			Name:      quay.GetName(),
			Namespace: quay.GetNamespace(),
		},
	)
	log.Info("checking Quay upgrade `Job` completion")

	nsn := types.NamespacedName{
		Name:      quay.GetName() + "-quay-app-upgrade",
		Namespace: quay.GetNamespace(),
	}
	var upgradeJob batchv1.Job
	if err := r.Client.Get(ctx, nsn, &upgradeJob); err != nil {
		log.Error(err, "could't retrieve Quay upgrade Job")
		return ctrl.Result{}, err
	}

	if upgradeJob.Status.Succeeded == 0 {
		log.Info("Quay upgrade `Job` not finished")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	log.Info("Upgrade complete, updating `status.currentVersion`")

	condition := v1.Condition{
		Type:               v1.ConditionTypeAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ConditionReasonHealthChecksPassing,
		Message:            "all registry component healthchecks passing",
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	quay.Status.Conditions = v1.SetCondition(quay.Status.Conditions, condition)
	quay.Status.CurrentVersion = v1.QuayVersionCurrent

	if err := r.Client.Status().Update(ctx, quay); err != nil {
		log.Error(err, "could not update QuayRegistry status with current version")
		return ctrl.Result{}, err
	}

	log.Info("successfully updated `status` after Quay upgrade")
	return ctrl.Result{}, nil
}

// Reconcile is called everytime an update or resync event happens in a QuayRegistry object.
func (r *QuayRegistryReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	r.Mtx.Lock()
	defer r.Mtx.Unlock()

	var err error
	log := r.Log.WithValues("quayregistry", req.NamespacedName)
	log.Info("begin reconcile")

	var quay v1.QuayRegistry
	if err = r.Client.Get(ctx, req.NamespacedName, &quay); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to retrieve QuayRegistry")
		return ctrl.Result{}, err
	}
	updatedQuay := quay.DeepCopy()
	quayContext := quaycontext.NewQuayRegistryContext()

	// if quay instance is flagged to be deleted get rid of related objects.
	if !quay.GetDeletionTimestamp().IsZero() {
		log.Info("QuayRegistry is flagged for deletion, finalizing it")
		return ctrl.Result{}, r.handleQuayRegistryDeletion(ctx, updatedQuay)
	}

	// if the migration is in progress we simply check the current status for the migration.
	available := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeAvailable)
	if available != nil && available.Reason == v1.ConditionReasonMigrationsInProgress {
		return r.verifyMigrationJob(ctx, updatedQuay)
	}

	// if there is no config bundle set create one with the default config and sets it in
	// the quay object. If this operation succeeds the QuayRegistry object will be updated
	// and a new event will eventually come in.
	if quay.Spec.ConfigBundleSecret == "" {
		log.Info("`spec.configBundleSecret` is unset. Creating base `Secret`")
		if err = r.createConfigBundle(ctx, updatedQuay); err != nil {
			return r.reconcileWithCondition(
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				err.Error(),
			)
		}
		log.Info("successfully updated `spec.configBundleSecret`")
		return ctrl.Result{}, nil
	}
	nsn := types.NamespacedName{
		Namespace: quay.GetNamespace(),
		Name:      quay.Spec.ConfigBundleSecret,
	}

	var configBundle corev1.Secret
	if err = r.Get(ctx, nsn, &configBundle); err != nil {
		msg := fmt.Sprintf(
			"unable to retrieve referenced `configBundleSecret`: %s, error: %s",
			quay.Spec.ConfigBundleSecret,
			err,
		)
		return r.reconcileWithCondition(
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			msg,
		)
	}

	// here we attempt to verify if the cluster supports all our dependencies such as
	// Routes, ObjectBucketClaims, etc. loadQuayRegistryContext loads information about
	// our dependencies into the provided QuayRegistryContext.
	if retryDelay, err := r.loadQuayRegistryContext(
		ctx, quayContext, updatedQuay, configBundle,
	); err != nil {
		if _, nerr := r.updateWithCondition(
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonObjectStorageComponentDependencyError,
			err.Error(),
		); err != nil {
			log.Error(nerr, "failed to update `conditions` of `QuayRegistry`")
		}
		return ctrl.Result{RequeueAfter: retryDelay}, nil
	}

	if updatedQuay, err = v1.EnsureDefaultComponents(quayContext, updatedQuay); err != nil {
		log.Error(err, "could not ensure default `spec.components`")
		return ctrl.Result{}, nil
	}

	if !v1.ComponentsMatch(quay.Spec.Components, updatedQuay.Spec.Components) {
		log.Info("updating QuayRegistry `spec.components` to include defaults")
		if err = r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `spec.components` to include defaults")
		}
		return ctrl.Result{}, nil
	}

	// verify now if we have all the needed configuration for the components. If a component
	// is not managed then the user must have provided its configuration.
	if err := r.validateComponentsConfig(updatedQuay, &configBundle); err != nil {
		return r.reconcileWithCondition(
			updatedQuay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			err.Error(),
		)
	}

	updatedQuay.Status.Conditions = v1.RemoveCondition(
		updatedQuay.Status.Conditions, v1.ConditionTypeRolloutBlocked,
	)

	log.Info("inflating QuayRegistry into Kubernetes objects using Kustomize")
	deploymentObjects, err := kustomize.Inflate(quayContext, updatedQuay, &configBundle, log)
	if err != nil {
		msg := fmt.Sprintf("could't inflate QuayRegistry into Kubernetes objects: %s", err)
		return r.reconcileWithCondition(
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonComponentCreationFailed,
			msg,
		)
	}

	// moves now to the actual objects creation.
	for _, obj := range kustomize.EnsureCreationOrder(deploymentObjects) {
		// For metrics and dashboards to work, we need to deploy the Grafana ConfigMap
		// in the `openshift-config-managed` namespace and add the label
		// `openshift.io/cluster-monitoring: true` to the registry namespace
		if quayContext.SupportsMonitoring && r.isGrafanaConfigMap(obj) {
			obj.SetNamespace(GrafanaDashboardConfigNamespace)
			if obj, err = r.updateGrafanaDashboardData(
				obj, updatedQuay.GetName(), updatedQuay.GetNamespace(),
			); err != nil {
				msg := fmt.Sprintf("Unable to update Grafana title %s", err)
				return r.reconcileWithCondition(
					&quay,
					v1.ConditionTypeRolloutBlocked,
					metav1.ConditionTrue,
					v1.ConditionReasonMonitoringComponentDependencyError,
					msg,
				)
			}
		}

		if err := r.createOrUpdateObject(ctx, obj, quay); err != nil {
			return r.reconcileWithCondition(
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonComponentCreationFailed,
				fmt.Sprintf("objects not created/updated successfully: %s", err),
			)
		}
	}

	if quayContext.SupportsMonitoring {
		if err := r.patchNamespaceForMonitoring(ctx, quay); err != nil {
			return r.reconcileWithCondition(
				updatedQuay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonMonitoringComponentDependencyError,
				err.Error(),
			)
		}
	}

	updatedQuay, _ = v1.EnsureConfigEditorEndpoint(quayContext, updatedQuay)
	configEditCredentials := configEditorCredentialsSecretFrom(deploymentObjects)
	updatedQuay.Status.ConfigEditorCredentialsSecret = configEditCredentials

	var rawcfg = configBundle.Data["config.yaml"]
	var userProvidedConfig map[string]interface{}
	if err := yaml.Unmarshal(rawcfg, &userProvidedConfig); err != nil {
		return r.reconcileWithCondition(
			updatedQuay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			err.Error(),
		)
	}

	// XXX checks if deploy is blocked due to invalid config and then does an update?
	blkcond := v1.GetCondition(updatedQuay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	if blkcond != nil && blkcond.Status == metav1.ConditionTrue {
		if blkcond.Reason == v1.ConditionReasonConfigInvalid {
			return r.reconcileWithCondition(
				updatedQuay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				blkcond.Message,
			)
		}
	}

	if updatedQuay, err = r.updateWithCondition(
		updatedQuay,
		v1.ConditionTypeRolloutBlocked,
		metav1.ConditionFalse,
		v1.ConditionReasonComponentsCreationSuccess,
		"all objects created/updated successfully",
	); err != nil {
		log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		return ctrl.Result{}, nil
	}

	managedOS := v1.ComponentIsManaged(updatedQuay.Spec.Components, "objectstorage")
	if managedOS && !quayContext.ObjectStorageInitialized {
		r.Log.Info("requeuing to populate values for managed component: `objectstorage`")
		return ctrl.Result{Requeue: true}, nil
	}

	// if the version has been updated there is a job in progress to migrate the database
	// to the new version, on this case just set the status to MigrationsInProgress and
	// return.
	if updatedQuay.Status.CurrentVersion != v1.QuayVersionCurrent {
		return r.reconcileWithCondition(
			updatedQuay,
			v1.ConditionTypeAvailable,
			metav1.ConditionFalse,
			v1.ConditionReasonMigrationsInProgress,
			"running database migrations",
		)
	}

	// we should be good to go, sets up the registry endpoints in the quay object and
	// make sure it contains the finalizer.
	updatedQuay, _ = v1.EnsureRegistryEndpoint(quayContext, updatedQuay, userProvidedConfig)

	if !controllerutil.ContainsFinalizer(updatedQuay, QuayOperatorFinalizer) {
		controllerutil.AddFinalizer(updatedQuay, QuayOperatorFinalizer)
		if err = r.Update(ctx, updatedQuay); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateGrafanaDashboardData parses the Grafana Dashboard ConfigMap and updates the title and
// labels.
func (r *QuayRegistryReconciler) updateGrafanaDashboardData(
	obj client.Object, quayName string, quayNamespace string,
) (client.Object, error) {
	updatedObj := obj.DeepCopyObject()
	configMapObj := updatedObj.(*corev1.ConfigMap)

	dashboardConfigJSON := configMapObj.Data[QuayDashboardJSONKey]

	newTitle := fmt.Sprintf("Quay - %s - %s", quayNamespace, quayName)
	dashboardConfigJSON, err := sjson.Set(dashboardConfigJSON, GrafanaTitleJSONPath, newTitle)
	if err != nil {
		return nil, err
	}

	if dashboardConfigJSON, err = sjson.Set(
		dashboardConfigJSON, GrafanaNamespaceFilterJSONPath, quayNamespace,
	); err != nil {
		return nil, err
	}

	metricsServiceName := fmt.Sprintf("%s-quay-metrics", quayName)
	if dashboardConfigJSON, err = sjson.Set(
		dashboardConfigJSON, GrafanaServiceFilterJSONPath, metricsServiceName,
	); err != nil {
		return nil, err
	}

	configMapObj.Data[QuayDashboardJSONKey] = dashboardConfigJSON
	return configMapObj, nil
}

// isGrafanaConfigMap checks if an Object is the Grafana ConfigMap used in the monitoring component
// returns a bool indicating if it is the expected ConfigMap.
func (r *QuayRegistryReconciler) isGrafanaConfigMap(obj client.Object) bool {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return false
	}
	return strings.HasSuffix(cm.GetName(), GrafanaDashboardConfigMapNameSuffix)
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)
	return yamlified
}

// createOrUpdateObject creates or updates provided object. It ensures that the object has provided
// QuayRegistry as its owner and also retries the creation in case of failure. The corner case here
// is the Grafana Dashboard config map, as it lives in a different namespace no ownership is
// established for it.
func (r *QuayRegistryReconciler) createOrUpdateObject(
	ctx context.Context, obj client.Object, quay v1.QuayRegistry,
) error {
	immutableResources := map[string]bool{
		schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}.String(): true,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}.String():  true,
	}

	// Remove owner reference from grafana dasbhoard config map to prevent cross-namespace
	// owner reference. Grafana config map lives in a different namespace. Ensure we have
	// owner reference in all other object kinds.
	if r.isGrafanaConfigMap(obj) {
		obj = v1.RemoveOwnerReference(&quay, obj)
	} else {
		obj = v1.EnsureOwnerReference(&quay, obj)
	}

	// managedFields cannot be set on a PATCH.
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{})

	groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()
	if immutableResources[groupVersionKind] {
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOptions := &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}
		if err := r.Client.Delete(
			ctx, obj, deleteOptions,
		); err != nil && !errors.IsNotFound(err) && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to delete immutable resource: %w", err)
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
			return fmt.Errorf("failed to create immutable resource: %w", err)
		}

		return nil
	}

	opts := []client.PatchOption{
		client.ForceOwnership, client.FieldOwner("quay-operator"),
	}
	if err := r.Client.Patch(ctx, obj, client.Apply, opts...); err != nil {
		return fmt.Errorf("failed to create/update object: %w", err)
	}
	return nil
}

// updateWithCondition appends/updates a condition in the provided QuayRegistry object. Updates
// remotely in the Kubernetes API and returns the updated version of the object.
func (r *QuayRegistryReconciler) updateWithCondition(
	q *v1.QuayRegistry,
	t v1.ConditionType,
	s metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) (*v1.QuayRegistry, error) {
	updatedQuay := q.DeepCopy()

	condition := v1.Condition{
		Type:               t,
		Status:             s,
		Reason:             reason,
		Message:            msg,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	updatedQuay.Status.Conditions = v1.SetCondition(q.Status.Conditions, condition)
	updatedQuay.Status.LastUpdate = time.Now().UTC().String()

	eventType := corev1.EventTypeNormal
	if s == metav1.ConditionTrue {
		eventType = corev1.EventTypeWarning
	}

	// FIXME: Need to pause here because race condition between updating `conditions`
	// multiple times changes `resourceVersion`... XXX (ricardo) this makes no sense.
	time.Sleep(1000 * time.Millisecond)

	// Fetch first to ensure we have the right `resourceVersion` for updates.
	nsn := types.NamespacedName{Namespace: q.GetNamespace(), Name: q.GetName()}
	var currentQuay v1.QuayRegistry
	if err := r.Client.Get(context.Background(), nsn, &currentQuay); err != nil {
		return nil, err
	}
	updatedQuay.SetResourceVersion(currentQuay.GetResourceVersion())

	if err := r.Client.Status().Update(context.Background(), updatedQuay); err != nil {
		return nil, err
	}

	// FIXME: Events are not being recorded during testing, making it hard to debug...
	r.EventRecorder.Event(updatedQuay, eventType, string(reason), msg)
	return updatedQuay, nil
}

// reconcileWithCondition sets the given condition on the `QuayRegistry` and returns a reconcile
// result.
func (r *QuayRegistryReconciler) reconcileWithCondition(
	q *v1.QuayRegistry,
	t v1.ConditionType,
	s metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) (ctrl.Result, error) {
	_, err := r.updateWithCondition(q, t, s, reason, msg)
	return ctrl.Result{}, err
}

// SetupWithManager initializes the controller manager. Register all needed schemes.
func (r *QuayRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add OpenShift `Route` API to scheme")
		return err
	}

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
		Complete(r)
}

// patchNamespaceForMonitoring makes sure the namespace where provided QuayRegistry lives has two
// labels: ClusterMonitoringLabelKey, and QuayOperatorManagedLabelKey.
func (r *QuayRegistryReconciler) patchNamespaceForMonitoring(
	ctx context.Context, quay v1.QuayRegistry,
) error {
	nsn := types.NamespacedName{Name: quay.GetNamespace()}
	var ns corev1.Namespace
	if err := r.Client.Get(ctx, nsn, &ns); err != nil {
		return err
	}

	monitoringLabel := ns.Labels[ClusterMonitoringLabelKey]
	managementLabel := ns.Labels[QuayOperatorManagedLabelKey]
	if monitoringLabel == "true" && managementLabel == "true" {
		return nil
	}

	updatedNS := ns.DeepCopy()
	updatedNS.Labels[ClusterMonitoringLabelKey] = "true"
	updatedNS.Labels[QuayOperatorManagedLabelKey] = "true"

	patch := client.MergeFrom(&ns)
	return r.Client.Patch(ctx, updatedNS, patch)
}

// cleanupNamespaceLabels removes ClusterMonitoringLabelKey and QuayOperatorManagedLabelKey labels
// from the namespace where provided QuayRegistry lives. Removes the labels only if there is only
// one QuayRegistry object living in the namespace and QuayOperatorManagedLabelKey is set.
func (r *QuayRegistryReconciler) cleanupNamespaceLabels(
	ctx context.Context, quay *v1.QuayRegistry,
) error {
	var ns corev1.Namespace
	err := r.Client.Get(ctx, types.NamespacedName{Name: quay.GetNamespace()}, &ns)
	if err != nil {
		return err
	}

	var list v1.QuayRegistryList
	listOps := client.ListOptions{
		Namespace: quay.GetNamespace(),
	}

	if err := r.Client.List(ctx, &list, &listOps); err != nil {
		return err
	}

	if ns.Labels == nil || ns.Labels[QuayOperatorManagedLabelKey] == "" {
		return nil
	}

	if len(list.Items) != 1 {
		return nil
	}

	updatedNs := ns.DeepCopy()
	delete(updatedNs.Labels, ClusterMonitoringLabelKey)
	delete(updatedNs.Labels, QuayOperatorManagedLabelKey)

	patch := client.MergeFrom(&ns)
	return r.Client.Patch(ctx, updatedNs, patch)
}

// cleanupGrafanaConfigMap removes the config map holding the graphana dashboard config
// for the provided QuayRegistry.
func (r *QuayRegistryReconciler) cleanupGrafanaConfigMap(
	ctx context.Context, quay *v1.QuayRegistry,
) error {
	cmname := types.NamespacedName{
		Name:      quay.GetName() + "-" + GrafanaDashboardConfigMapNameSuffix,
		Namespace: GrafanaDashboardConfigNamespace,
	}

	var cm corev1.ConfigMap
	if err := r.Client.Get(ctx, cmname, &cm); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return r.Client.Delete(ctx, &cm)
}

// finalizeQuay acts to remove everything that is linked to a QuayRegistry object.
func (r *QuayRegistryReconciler) finalizeQuay(ctx context.Context, quay *v1.QuayRegistry) error {
	// NOTE: `controller-runtime` hangs rather than return "forbidden" error if insufficient
	// RBAC permissions, so we use `WatchNamespace` to skip:
	// https://github.com/kubernetes-sigs/controller-runtime/issues/550
	if r.WatchNamespace != "" {
		r.Log.Info("skipping finalizer in all-namespaces mode: namespace label cleanup")
		return nil
	}

	r.Log.Info("cleaning up namespace labels")
	if err := r.cleanupNamespaceLabels(ctx, quay); err != nil {
		return err
	}
	r.Log.Info("successfully cleaned up namespace labels")

	r.Log.Info("cleaning up Grafana `ConfigMap`")
	if err := r.cleanupGrafanaConfigMap(ctx, quay); err != nil {
		return err
	}
	r.Log.Info("successfully cleaned up grafana config map")

	return nil
}
