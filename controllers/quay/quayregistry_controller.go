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
	"sync"
	"time"

	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const (
	creationPollInterval = time.Second * 2
	creationPollTimeout  = time.Second * 600

	grafanaDashboardConfigMapNameSuffix = "grafana-dashboard-quay"
	grafanaTitleJSONPath                = "title"
	grafanaNamespaceFilterJSONPath      = "templating.list.1.options.0.value"
	grafanaServiceFilterJSONPath        = "templating.list.2.options.0.value"
	clusterMonitoringLabelKey           = "openshift.io/cluster-monitoring"
	quayDashboardJSONKey                = "quay.json"
	quayOperatorManagedLabelKey         = "quay-operator/managed-label"
	quayOperatorFinalizer               = "quay-operator/finalizer"
	grafanaDashboardConfigNamespace     = "openshift-config-managed"
)

// QuayRegistryReconciler reconciles a QuayRegistry object
type QuayRegistryReconciler struct {
	client.Client
	Log                  logr.Logger
	Scheme               *runtime.Scheme
	EventRecorder        record.EventRecorder
	WatchNamespace       string
	Mtx                  *sync.Mutex
	Requeue              ctrl.Result
	SkipResourceRequests bool
}

// handleFlaggedForDeletion makes sure that we process a QuayRegistry once it is flagged for
// deletion. Removes the finalizer once it is done, requeues an event for the registry
// in case of failure.
func (r *QuayRegistryReconciler) handleFlaggedForDeletion(
	ctx context.Context, quay *v1.QuayRegistry, log logr.Logger,
) (ctrl.Result, error) {
	log.Info("`QuayRegistry` to be deleted")
	if !controllerutil.ContainsFinalizer(quay, quayOperatorFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.finalizeQuay(ctx, quay); err != nil {
		return r.Requeue, err
	}

	controllerutil.RemoveFinalizer(quay, quayOperatorFinalizer)
	if err := r.Update(ctx, quay); err != nil {
		return r.Requeue, err
	}

	return ctrl.Result{}, nil
}

// handleMigrationsRunning checks the migration job status for a given QuayRegistry instance.
// This function verifies if the job ran successfully and sets its condition properly. The
// result of this function is always a ctrl.Result with a proper reschedule value. Once
// migration job has been finished this function sets quay.Status.CurrentVersion to the
// value of v1.QuayVersionCurrent, indicating that we have migrated to the current version.
func (r *QuayRegistryReconciler) handleMigrationsRunning(
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
			return r.Requeue, nil
		}

		log.Error(err, "could not retrieve Quay upgrade `Job`")
		return r.Requeue, nil
	}

	if job.Status.Active == 1 {
		log.Info("Upgrade job running, requeueing reconcile...")
		return r.Requeue, nil
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
			return r.Requeue, nil
		}

		log.Info("successfully updated `status` after Quay upgrade")
		return r.Requeue, nil
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

	return r.Requeue, nil
}

// handleNeedsBundleSecret creates a new config bundle secret for provided QuayRegistry
// object, the created bundle contains a default quay config and is then populated as
// ConfigBundleSecret. QuayRegistry is updated and a reschedule Result is returned.
func (r *QuayRegistryReconciler) handleNeedsBundleSecret(
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
				"config.yaml": encode(kustomize.BaseQuayConfig()),
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
		return r.Requeue, err
	}

	log.Info("successfully updated `spec.configBundleSecret`")
	return r.Requeue, nil
}

// getConfigBundleSecret returns the secret used to configure provided QuayRegistry
// instance.
func (r *QuayRegistryReconciler) getConfigBundleSecret(
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

// Reconcile is called every time an update happens in a QuayRegistry object. It attempts to
// create all needed objects to get a quay instance running.
func (r *QuayRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Mtx.Lock()
	defer r.Mtx.Unlock()

	regid := fmt.Sprintf("%s/%s", req.NamespacedName.Namespace, req.NamespacedName.Name)
	log := r.Log.WithValues("quayregistry", regid)
	log.Info("begin reconcile")

	var quay v1.QuayRegistry
	if err := r.Client.Get(ctx, req.NamespacedName, &quay); err != nil {
		if errors.IsNotFound(err) {
			log.Info("`QuayRegistry` deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to retrieve QuayRegistry")
		return r.Requeue, nil
	}

	quayRef := quay.DeepCopy()

	if v1.FlaggedForDeletion(quayRef) {
		return r.handleFlaggedForDeletion(ctx, quayRef, log)
	}

	if v1.MigrationsRunning(quayRef) {
		return r.handleMigrationsRunning(ctx, quayRef, log)
	}

	if v1.NeedsBundleSecret(quayRef) {
		return r.handleNeedsBundleSecret(ctx, quayRef, log)
	}

	if err := v1.ValidateOverrides(quayRef); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonComponentOverrideInvalid,
			fmt.Sprintf("invalid overrides: %s", err),
		)
	}
	configBundleSecretRef, err := r.getConfigBundleSecret(ctx, quayRef)
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
	if err := r.validateHasNecessaryConfig(quay, configBundleSecretRef.Data); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			err.Error(),
		)
	}

	// Create context to infer things about QuayRegistry
	quayContext := quaycontext.NewQuayRegistryContext()

	if err := r.setTLSContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("Could not get TLS: %s", err),
		)
	}

	if err := r.setManagedKeysContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("unable to retrieve managed keys `Secret`: %s", err),
		)
	}

	if err := r.setRouteContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonRouteComponentDependencyError,
			fmt.Sprintf("could not check for `Routes` API: %s", err),
		)
	}

	if v1.ComponentIsManaged(quayRef.Spec.Components, v1.ComponentObjectStorage) {
		if err := r.setObjectStorageContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
			return r.reconcileWithCondition(
				ctx,
				&quay,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonObjectStorageComponentDependencyError,
				fmt.Sprintf("error checking for object storage support: %s", err),
			)
		}
	}

	if err := r.setBuildManagerContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			fmt.Sprintf("could not check for build manager support: %s", err),
		)
	}

	if v1.ComponentIsManaged(quayRef.Spec.Components, v1.ComponentMonitoring) {
		if err := r.setMonitoringContext(ctx, quayContext, quayRef, configBundleSecretRef); err != nil {
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

	if err = v1.EnsureDefaultComponents(quayContext, quayRef); err != nil {
		log.Error(err, "could not ensure default `spec.components`")
		return r.Requeue, err
	}

	if !v1.ComponentsMatch(quay.Spec.Components, quayRef.Spec.Components) {
		log.Info("updating QuayRegistry `spec.components` to include defaults")
		if err = r.Client.Update(ctx, quayRef); err != nil {
			log.Error(err, "failed to update `spec.components` to include defaults")
		}
		return r.Requeue, nil
	}

	var usercfg map[string]interface{}
	if err = yaml.Unmarshal(configBundleSecretRef.Data["config.yaml"], &usercfg); err != nil {
		return r.reconcileWithCondition(
			ctx,
			&quay,
			v1.ConditionTypeRolloutBlocked,
			metav1.ConditionTrue,
			v1.ConditionReasonConfigInvalid,
			err.Error(),
		)
	}

	quayRef.Status.Conditions = v1.RemoveCondition(
		quayRef.Status.Conditions, v1.ConditionTypeRolloutBlocked,
	)

	log.Info("inflating QuayRegistry into Kubernetes objects")
	deploymentObjects, err := kustomize.Inflate(
		quayContext, quayRef, configBundleSecretRef, log, r.SkipResourceRequests,
	)
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
			obj.SetNamespace(grafanaDashboardConfigNamespace)
			if err = updateGrafanaDashboardData(obj, quayRef); err != nil {
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

		if err := r.createOrUpdateObject(ctx, obj, quay, log); err != nil {
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
				quayRef,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonMonitoringComponentDependencyError,
				err.Error(),
			)
		}
	}

	v1.EnsureConfigEditorEndpoint(quayContext, quayRef)
	cfgsecret := configEditorCredentialsSecretFrom(deploymentObjects)
	quayRef.Status.ConfigEditorCredentialsSecret = cfgsecret

	rolloutBlocked := v1.GetCondition(
		quayRef.Status.Conditions,
		v1.ConditionTypeRolloutBlocked,
	)
	if rolloutBlocked != nil {
		invalid := rolloutBlocked.Reason == v1.ConditionReasonConfigInvalid
		if invalid && rolloutBlocked.Status == metav1.ConditionTrue {
			return r.reconcileWithCondition(
				ctx,
				quayRef,
				v1.ConditionTypeRolloutBlocked,
				metav1.ConditionTrue,
				v1.ConditionReasonConfigInvalid,
				rolloutBlocked.Message,
			)
		}
	}

	if err := r.updateWithCondition(
		ctx,
		quayRef,
		v1.ConditionTypeRolloutBlocked,
		metav1.ConditionFalse,
		v1.ConditionReasonComponentsCreationSuccess,
		"All objects created/updated successfully",
	); err != nil {
		log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		return r.Requeue, nil
	}

	if v1.ComponentIsManaged(quayRef.Spec.Components, "objectstorage") && !quayContext.ObjectStorageInitialized {
		r.Log.Info("requeuing to populate values for managed component: `objectstorage`")
		return r.Requeue, nil
	}

	upToDate := v1.EnsureRegistryEndpoint(quayContext, quayRef, usercfg)
	if !upToDate {
		if err = r.Client.Status().Update(ctx, quayRef); err != nil {
			log.Error(err, "failed to update `registryEndpoint` of `QuayRegistry`")
			return r.Requeue, nil
		}
	}

	// if the version differ then it means that the operator was upgraded and we need
	// to wait until the database upgrade job finishes. sets a condition here and
	// returns.
	if quayRef.Status.CurrentVersion != v1.QuayVersionCurrent {
		if err := r.updateWithCondition(
			ctx,
			quayRef,
			v1.ConditionComponentsCreated,
			metav1.ConditionFalse,
			v1.ConditionReasonMigrationsInProgress,
			"running database migrations",
		); err != nil {
			log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		}
		return r.Requeue, nil
	}

	if !controllerutil.ContainsFinalizer(quayRef, quayOperatorFinalizer) {
		controllerutil.AddFinalizer(quayRef, quayOperatorFinalizer)
		if err := r.Update(ctx, quayRef); err != nil {
			return r.Requeue, err
		}
	}

	// when we get to this point all objects were created as expected and we can safely
	// increase our reconcile delay.
	return ctrl.Result{RequeueAfter: time.Minute}, nil
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
		For(&v1.QuayRegistry{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		// TODO: Add `.Owns()` for every resource type we manage...
		Complete(r)
}
