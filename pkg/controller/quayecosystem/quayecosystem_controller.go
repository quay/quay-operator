package quayecosystem

import (
	"context"
	"math"
	"time"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/provisioning"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/setup"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"
	"github.com/redhat-cop/quay-operator/pkg/k8sutils"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new QuayEcosystem Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {

	k8sclient, err := k8sutils.GetK8sClient(mgr.GetConfig())

	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr, k8sclient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, k8sclient kubernetes.Interface) reconcile.Reconciler {

	reconcilerBase := util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("quayecosystem-controller"))

	return &ReconcileQuayEcosystem{reconcilerBase: reconcilerBase, k8sclient: k8sclient, quaySetupManager: setup.NewQuaySetupManager(reconcilerBase, k8sclient)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("quayecosystem-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource QuayEcosystem
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.QuayEcosystem{}}, &handler.EnqueueRequestForObject{}, util.ResourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileQuayEcosystem implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileQuayEcosystem{}

// ReconcileQuayEcosystem reconciles a QuayEcosystem object
type ReconcileQuayEcosystem struct {
	reconcilerBase   util.ReconcilerBase
	k8sclient        kubernetes.Interface
	quaySetupManager *setup.QuaySetupManager
}

// Reconcile performs the primary reconciliation loop
func (r *ReconcileQuayEcosystem) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logging.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling QuayEcosystem")

	// Fetch the Quay instance
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, quayEcosystem)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Initialize a new Quay Configuration Resource
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Initialize Configuration
	configuration := provisioning.New(r.reconcilerBase, r.k8sclient, &quayConfiguration)
	metaObject := resources.NewResourceObjectMeta(quayConfiguration.QuayEcosystem)

	// Set default values
	changed := validation.SetDefaults(r.reconcilerBase.GetClient(), &quayConfiguration)

	if changed {

		err := r.reconcilerBase.GetClient().Update(context.TODO(), quayConfiguration.QuayEcosystem)

		if err != nil {
			logging.Log.Error(err, "Failed to update QuayEcosystem after setting defaults")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)

		}

		_, err = r.manageSuccess(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemUpdateDefaultConfigurationConditionSuccess, "", "Configuration Updated Successfully")

		if err != nil {
			logging.Log.Error(err, "Failed to update QuayEcosystem status after setting defaults")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemUpdateDefaultConfigurationConditionFailure, err)
		}

		return reconcile.Result{}, nil

	}

	// Validate Configuration
	valid, err := validation.Validate(r.reconcilerBase.GetClient(), &quayConfiguration)
	if err != nil {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemValidationFailure, err)
	}
	if !valid {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemValidationFailure, err)
	}

	result, err := configuration.CoreResourceDeployment(metaObject)
	if err != nil {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
	}

	if result != nil {
		return *result, nil
	}

	result, err = configuration.ManageQuayEcosystemCertificates(metaObject)

	if err != nil {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
	}

	if result != nil {
		return *result, nil
	}

	if quayConfiguration.DeployQuayConfiguration {

		deployQuayConfigResult, err := configuration.DeployQuayConfiguration(metaObject)
		if err != nil {
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
		}

		if deployQuayConfigResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to deploy Quay config", "Failed to deploy Quay config")
			return *deployQuayConfigResult, nil
		}
	}

	// Execute setup if it has not been completed
	if !quayConfiguration.QuayEcosystem.Status.SetupComplete && !quayConfiguration.QuayEcosystem.Spec.Quay.SkipSetup {

		// Wait 5 seconds prior to kicking off setup
		time.Sleep(time.Duration(5) * time.Second)

		err = r.quaySetupManager.PrepareForSetup(r.reconcilerBase.GetClient(), &quayConfiguration)

		if err != nil {
			logging.Log.Error(err, "Failed to prepare for Quay Setup")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupFailure, err)
		}

		quaySetupInstance, err := r.quaySetupManager.NewQuaySetupInstance(&quayConfiguration)

		if err != nil {
			logging.Log.Error(err, "Failed to obtain QuaySetupInstance")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupFailure, err)
		}

		err = r.quaySetupManager.SetupQuay(quaySetupInstance)

		if err != nil {
			logging.Log.Error(err, "Failed to Setup Quay")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupFailure, err)
		}

		// Update flags when setup is completed
		quayConfiguration.QuayEcosystem.Status.SetupComplete = true

		// Reset the Config Deployment flag to the default value after successful setup
		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment) || !quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment {
			quayConfiguration.DeployQuayConfiguration = false
		}

		_, err = r.manageSuccess(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupSuccess, "", "Setup Completed Successfully")
		if err != nil {
			logging.Log.Error(err, "Failed to update QuayEcosystem status after Quay Setup Completion")
			return reconcile.Result{}, err
		}

	}

	deployQuayResult, err := configuration.DeployQuay(metaObject)
	if err != nil {
		r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Deploy Quay", err.Error())
		return reconcile.Result{}, err
	}

	if deployQuayResult != nil {
		r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Deploy Quay", "Failed to Deploy Quay")
		return *deployQuayResult, nil
	}

	if !quayConfiguration.DeployQuayConfiguration {

		removeQuayConfigResult, err := configuration.RemoveQuayConfigResources(metaObject)

		if err != nil {
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
		}

		if removeQuayConfigResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Remove Quay Config", "Failed to Remove Quay Config")
			return *removeQuayConfigResult, nil
		}

	}

	return reconcile.Result{}, nil

}

func (r *ReconcileQuayEcosystem) manageSuccess(instance *redhatcopv1alpha1.QuayEcosystem, conditionType redhatcopv1alpha1.QuayEcosystemConditionType, reason string, message string) (reconcile.Result, error) {

	condition := redhatcopv1alpha1.QuayEcosystemCondition{
		Type:    conditionType,
		Reason:  reason,
		Message: message,
		Status:  corev1.ConditionTrue,
	}

	instance.SetCondition(condition)

	err := r.reconcilerBase.GetClient().Status().Update(context.TODO(), instance)

	if err != nil {
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileQuayEcosystem) manageError(instance *redhatcopv1alpha1.QuayEcosystem, conditionType redhatcopv1alpha1.QuayEcosystemConditionType, issue error) (reconcile.Result, error) {

	r.reconcilerBase.GetRecorder().Event(instance, "Warning", "ProcessingError", issue.Error())

	existingCondition, found := instance.FindConditionByType(conditionType)

	lastUpdate := existingCondition.LastUpdateTime

	if !found {
		lastUpdate = metav1.NewTime(time.Now())
	}

	condition := redhatcopv1alpha1.QuayEcosystemCondition{
		Type:    conditionType,
		Reason:  "ProcessingError",
		Message: issue.Error(),
		Status:  corev1.ConditionFalse,
	}

	instance.SetCondition(condition)

	err := r.reconcilerBase.GetClient().Status().Update(context.TODO(), instance)

	if err != nil {
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}

	var retryInterval time.Duration
	if !found || existingCondition.Status == corev1.ConditionTrue {
		retryInterval = time.Second
	} else {
		retryInterval = time.Now().Sub(lastUpdate.Time).Round(time.Second)
	}

	requeue := time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6)))

	return reconcile.Result{
		RequeueAfter: requeue,
		Requeue:      true,
	}, nil
}
