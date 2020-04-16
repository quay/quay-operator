package quayecosystem

import (
	"context"
	"math"
	"reflect"
	"time"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"gopkg.in/yaml.v3"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/externalaccess"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/provisioning"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/quayconfig"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/setup"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"
	"github.com/redhat-cop/quay-operator/pkg/k8sutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
			logging.Log.Error(resourcesErr, "Error Determining Whether Quay Operator Running in OpenShift")
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
		QuayEcosystem:              quayEcosystem,
		IsOpenShift:                r.isOpenShift,
		RequiredSCCServiceAccounts: []string{utils.MakeServiceAccountUsername(quayEcosystem.Namespace, constants.QuayServiceAccount)},
	}

	// Initialize Configuration
	configuration := provisioning.New(r.reconcilerBase, r.k8sclient, &quayConfiguration)
	metaObject := resources.NewResourceObjectMeta(quayConfiguration.QuayEcosystem)

	// QuayEcosystem object is being deleted
	if util.IsBeingDeleted(quayEcosystem) {
		logging.Log.Info("QuayEcosystem Object Being Deleted. Cleaning up")

		if r.isOpenShift == true {
			if err := configuration.ConfigureAnyUIDSCCs(metaObject, utils.MakeServiceAccountsUsername(quayEcosystem.Namespace, constants.QuayEcosystemServiceAccounts), constants.OperationRemove); err != nil {
				logging.Log.Error(err, "Failed to Remove Users from SCCs")
				return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemUpdateDefaultConfigurationConditionFailure, err)
			}
		}

		if util.HasFinalizer(quayEcosystem, constants.OperatorFinalizer) {

			util.RemoveFinalizer(quayEcosystem, constants.OperatorFinalizer)

			err := r.reconcilerBase.GetClient().Update(context.TODO(), quayConfiguration.QuayEcosystem)

			if err != nil {
				logging.Log.Error(err, "Failed to update QuayEcosystem after finalizer removal")
				return r.manageError(quayConfiguration.QuayEcosystem, redhatcopv1alpha1.QuayEcosystemCleanupFailure, err)
			}
		}

		return reconcile.Result{}, nil
	}

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

	switch quayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.Type {
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
		external = &externalaccess.NodePortExternalAccess{
			QuayConfiguration: &quayConfiguration,
			ReconcilerBase:    r.reconcilerBase,
		}
	case redhatcopv1alpha1.IngressExternalAccessType:
		external = &externalaccess.IngressExternalAccess{
			QuayConfiguration: &quayConfiguration,
			ReconcilerBase:    r.reconcilerBase,
		}
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

	if !quayConfiguration.QuayEcosystem.Status.SetupComplete || quayConfiguration.DeployQuayConfiguration {
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

	// Manage Config Secret

	_, err = configuration.SyncQuayConfigSecret(metaObject)
	if err != nil {
		r.reconcilerBase.GetRecorder().Event(quayConfiguration.QuayEcosystem, "Warning", "Failed to Synchronize the Quay Config Secret", err.Error())
		return reconcile.Result{}, err
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

	// Reconcile Quay configuration specified in the CR.
	// TODO: This controller is too long. Refactor and clean it up.
	// NOTE: This is a naive implementation for the sake of time and simplicity.
	//       A cleaner implementation should be designed and implemented in the
	//       future.
	if quayConfiguration.QuayEcosystem.Status.SetupComplete {

		logging.Log.Info("Reconciling Quay configuration.")

		// Fetch the configuration as specified in the CR
		db := quayconfig.DatabaseConfig{
			Host:     quayConfiguration.QuayDatabase.Server,
			Username: quayConfiguration.QuayDatabase.Username,
			Password: quayConfiguration.QuayDatabase.Password,
			Name:     quayConfiguration.QuayDatabase.Database,
		}

		redis := quayconfig.RedisConfig{
			Host:     quayConfiguration.RedisHostname,
			Password: quayConfiguration.RedisPassword,
			Port:     int(*quayConfiguration.RedisPort),
		}

		desiredConfig := quayconfig.InfrastructureConfig{
			Database:   db,
			Redis:      redis,
			Hostname:   quayConfiguration.QuayHostname,
			Superusers: quayConfiguration.QuayEcosystem.Spec.Quay.Superusers,
		}

		// Fetch Quay's `config.yaml` from its configuration secret
		secret := &corev1.Secret{}
		target := types.NamespacedName{
			Namespace: request.Namespace,
			Name:      "quay-enterprise-config-secret", // TODO: Should not be hard-coded
		}
		err := r.reconcilerBase.GetClient().Get(context.TODO(), target, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				logging.Log.Error(err, "Unable to find Quay's configuration secret.")
				return reconcile.Result{}, nil
			}
			logging.Log.Error(err, "Unable to fetch Quay's configuration secret.")
			return reconcile.Result{}, err
		}

		// Verify and reconcile all configuration values managed by the operator
		if fileContents, ok := secret.Data["config.yaml"]; ok {

			// Unmarshall those values managed by the Operator
			persistedConfig := &quayconfig.ConfigFile{}
			err = yaml.Unmarshal(fileContents, persistedConfig)
			if err != nil {
				logging.Log.Error(err, "Unable to deserialize configuration file.")
				return reconcile.Result{}, err
			}

			// Unmarshall the rest of the file
			// NOTE: This is done because the operator does not have a defined
			//       struct which represents the entire config.yaml schema. It
			//       also avoids unintentionally adding new fields to the
			//       configuration file.
			if persistedConfig.NotManagedByOperator == nil {
				persistedConfig.NotManagedByOperator = make(map[string]interface{})
			}
			err = yaml.Unmarshal(fileContents, persistedConfig.NotManagedByOperator)
			if err != nil {
				logging.Log.Error(err, "Unable to deserialize configuration file.")
				return reconcile.Result{}, err
			}

			// Reconcile Hostname Changes
			if persistedConfig.Hostname != desiredConfig.Hostname {

				logging.Log.Info("Quay's Hostname has changed. Reconciling.")

				configFileKey := "SERVER_HOSTNAME"

				// Update the Hostname in the internal config.yaml representation
				persistedConfig.NotManagedByOperator[configFileKey] = desiredConfig.Hostname
				data, err := yaml.Marshal(persistedConfig.NotManagedByOperator)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's Hostname.")
					return reconcile.Result{}, err
				}

				// Update the `config.yaml` stored in the secret used by Quay
				secret.Data["config.yaml"] = data
				err = r.reconcilerBase.GetClient().Update(context.Background(), secret)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's Hostname configuration.")
					return reconcile.Result{}, err
				}

				logging.Log.Info("Updated Quay's Hostname configuration.")

			} else {
				logging.Log.Info("Quay's Hostname is correct. No changes needed.")
			}

			// Reconcile Superusers
			if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Superusers) && len(quayConfiguration.QuayEcosystem.Spec.Quay.Superusers) > 0 && !reflect.DeepEqual(persistedConfig.Superusers, desiredConfig.Superusers) {
				logging.Log.Info("Superusers have changed. Reconciling.")

				configFileKey := "SUPER_USERS"

				// Update the Hostname in the internal config.yaml representation
				persistedConfig.NotManagedByOperator[configFileKey] = desiredConfig.Superusers
				data, err := yaml.Marshal(persistedConfig.NotManagedByOperator)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's Superusers.")
					return reconcile.Result{}, err
				}

				// Update the `config.yaml` stored in the secret used by Quay
				secret.Data["config.yaml"] = data
				err = r.reconcilerBase.GetClient().Update(context.Background(), secret)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's superuser configuration.")
					return reconcile.Result{}, err
				}

				logging.Log.Info("Updated Quay's superuser configuration.")
			} else {
				logging.Log.Info("Superusers are correct. No changes needed.")
			}

			// Reconcile Redis Changes
			// TODO: This conditional statement is too long. Refactor.
			if persistedConfig.Redis.Host != desiredConfig.Redis.Host || persistedConfig.Redis.Port != desiredConfig.Redis.Port || persistedConfig.Redis.Password != desiredConfig.Redis.Password {

				logging.Log.Info("Redis has changed. Reconciling.")
				configFileKey := "USER_EVENTS_REDIS"

				// Update the Redis Configuration in the internal config.yaml representation
				persistedConfig.NotManagedByOperator[configFileKey] = desiredConfig.Redis
				data, err := yaml.Marshal(persistedConfig.NotManagedByOperator)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Redis Configuration.")
				}

				// Update the `config.yaml` stored in the secret used by Quay
				secret.Data["config.yaml"] = data
				err = r.reconcilerBase.GetClient().Update(context.Background(), secret)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Redis configuration.")
					return reconcile.Result{}, err
				}

				logging.Log.Info("Updated Quay's Redis configuration.")

			} else {
				logging.Log.Info("Redis configuration is correct. No changes needed.")
			}

			// Reconcile Database Changes
			desiredConnectionString, err := desiredConfig.Database.ToConnectionString()
			if err != nil {
				logging.Log.Error(err, "Unable to construct Database connection string.")
				return reconcile.Result{}, err
			}

			if persistedConfig.DatabaseURI != desiredConnectionString {

				logging.Log.Info("Quay's Database configuration has changed. Reconciling.")
				configFileKey := "DB_URI"

				// Update the Redis Configuration in the internal config.yaml representation
				persistedConfig.NotManagedByOperator[configFileKey] = desiredConnectionString
				data, err := yaml.Marshal(persistedConfig.NotManagedByOperator)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's Database Configuration.")
				}

				// Update the `config.yaml` stored in the secret used by Quay
				secret.Data["config.yaml"] = data
				err = r.reconcilerBase.GetClient().Update(context.Background(), secret)
				if err != nil {
					logging.Log.Error(err, "Unable to reconcile Quay's Database configuration.")
					return reconcile.Result{}, err
				}

				logging.Log.Info("Updated Quay's Database configuration.")

			} else {
				logging.Log.Info("Quay's Database configuration is correct. No changes needed.")
			}

		} else {
			msg := "Unable to reconcile Quay configuration. Cannot access `config.yaml`."
			logging.Log.Error(nil, msg)
			return reconcile.Result{}, nil
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
