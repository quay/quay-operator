package quayecosystem

import (
	"context"
	"math"
	"time"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/externalaccess"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/provisioning"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/setup"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"
	"github.com/redhat-cop/quay-operator/pkg/k8sutils"
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

	reconcilerBase := util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor("quayecosystem-controller"))

	discoveryClient, _ := reconcilerBase.GetDiscoveryClient()

	// Query for known OpenShift API resource to verify it is available
	_, resourcesErr := discoveryClient.ServerResourcesForGroupVersion("security.openshift.io/v1")

	isOpenShift := true

	if resourcesErr != nil {
		if errors.IsNotFound(resourcesErr) {
			isOpenShift = false
		} else {
			logging.Log.Error(resourcesErr, "Error Determing Whether Quay Operator Running in OpenShift")
		}
	}

	return &ReconcileQuayEcosystem{reconcilerBase: reconcilerBase, k8sclient: k8sclient, quaySetupManager: setup.NewQuaySetupManager(reconcilerBase, k8sclient), isOpenShift: isOpenShift}
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
	isOpenShift      bool
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
		IsOpenShift:   r.isOpenShift,
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

	}

	// Validate Configuration
	valid, err := validation.Validate(r.reconcilerBase.GetClient(), &quayConfiguration)
	if err != nil {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemValidationFailure, err)
	}
	if !valid {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemValidationFailure, err)
	}

	// Instantiate External Access
	var external externalaccess.ExternalAccess

	switch quayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccessType {
	case redhatcopv1alpha1.RouteExternalAccessType:
		external = &externalaccess.RouteExternalAccess{
			QuayConfiguration: &quayConfiguration,
			ReconcilerBase:    r.reconcilerBase,
		}
	case redhatcopv1alpha1.LoadBalancerExternalAccessType:
		external = &externalaccess.LoadBalancerExternalAccess{
			QuayConfiguration: &quayConfiguration,
			K8sClient:         r.k8sclient,
		}
	case redhatcopv1alpha1.NodePortExternalAccessType:
		external = &externalaccess.NodePortExternalAccess{}
	}

	result, err := configuration.CoreQuayResourceDeployment(metaObject)
	if err != nil {
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
	}

	if result != nil {
		return *result, nil
	}

	// Manage Quay and QuayConfig External Access
	if err := external.ManageQuayExternalAccess(metaObject); err != nil {
		logging.Log.Error(err, "Failed to Setup Quay External Access")
		return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ConfigHostname) && quayConfiguration.DeployQuayConfiguration {
		if err := external.ManageQuayConfigExternalAccess(metaObject); err != nil {
			logging.Log.Error(err, "Failed to Setup Quay Config External Access")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
		}
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
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to deploy Quay config", err.Error())
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

		quaySetupInstance, err := r.quaySetupManager.NewQuaySetupInstance(&quayConfiguration)

		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to obtain QuaySetupInstance", err.Error())
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupFailure, err)
		}

		err = r.quaySetupManager.SetupQuay(quaySetupInstance)

		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Setup Quay", err.Error())
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemQuaySetupFailure, err)
		}

		// Update flags when setup is completed
		quayConfiguration.QuayEcosystem.Status.SetupComplete = true

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

	// Deploy Quay Repo Mirror
	if quayConfiguration.QuayEcosystem.Spec.Quay.EnableRepoMirroring {

		deployQuayRepoMirrorResult, err := configuration.DeployQuayRepoMirror(metaObject)
		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to deploy Quay Repo Mirror", err.Error())
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemProvisioningFailure, err)
		}

		if deployQuayRepoMirrorResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to deploy Quay Repo Mirror", "Failed to deploy Quay Repo Mirror")
			return *deployQuayRepoMirrorResult, nil
		}
	}

	// Manage Clair Resources
	if quayConfiguration.QuayEcosystem.Spec.Clair != nil && quayConfiguration.QuayEcosystem.Spec.Clair.Enabled {

		// Setup Security Scanner
		configureSecurityScannerResult, err := configuration.ConfigureSecurityScanner(metaObject)
		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Configure Security Scanner", err.Error())
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemSecurityScannerConfigurationFailure, err)
		}

		if configureSecurityScannerResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Configure Security Scanner", "Failed to Configure Security Scanner")
			return *configureSecurityScannerResult, nil
		}

		_, err = r.manageSuccess(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemClairConfigurationSuccess, "", "Clair Configuration Updated Successfully")

		if err != nil {
			logging.Log.Error(err, "Failed to update QuayEcosystem after security scanner completion")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemSecurityScannerConfigurationFailure, err)
		}

		// Clair components
		manageClairResourceResult, err := configuration.ManageClairComponents(metaObject)
		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Configure Clair", err.Error())
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemClairConfigurationFailure, err)
		}

		if manageClairResourceResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Configure Clair", "Failed to Configure Clair")
			return *manageClairResourceResult, nil
		}

		_, err = r.manageSuccess(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemClairConfigurationSuccess, "", "Clair Configuration Updated Successfully")

		if err != nil {
			logging.Log.Error(err, "Failed to update QuayEcosystem after Clair configuration success")
			return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemClairConfigurationFailure, err)
		}

		deployClairResult, err := configuration.DeployClair(metaObject)
		if err != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Deploy Clair", err.Error())
			return reconcile.Result{}, err
		}

		if deployClairResult != nil {
			r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Deploy Clair", "Failed to Deploy Clair")
			return *deployClairResult, nil
		}

	}

	// Determine if Config pod should be spun down
	// Reset the Config Deployment flag to the default value after successful setup
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment) || !quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment {
		quayConfiguration.DeployQuayConfiguration = false
	}

	// Spin down the config pod
	if !quayConfiguration.DeployQuayConfiguration {

		removeQuayConfigResult, err := configuration.RemoveQuayConfigResources(metaObject, external)

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
