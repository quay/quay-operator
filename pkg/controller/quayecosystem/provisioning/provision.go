package provisioning

import (
	"context"
	"fmt"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	ossecurityv1 "github.com/openshift/api/security/v1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redhat-cop/quay-operator/pkg/k8sutils"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-cop/operator-utils/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileQuayEcosystemConfiguration defines values required for Quay configuration
type ReconcileQuayEcosystemConfiguration struct {
	reconcilerBase    util.ReconcilerBase
	k8sclient         kubernetes.Interface
	quayConfiguration *resources.QuayConfiguration
}

// New creates the structure for the Quay configuration
func New(reconcilerBase util.ReconcilerBase, k8sclient kubernetes.Interface,
	quayConfiguration *resources.QuayConfiguration) *ReconcileQuayEcosystemConfiguration {
	return &ReconcileQuayEcosystemConfiguration{
		reconcilerBase:    reconcilerBase,
		k8sclient:         k8sclient,
		quayConfiguration: quayConfiguration,
	}
}

// CoreResourceDeployment takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) CoreResourceDeployment(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.createQuayConfigSecret(metaObject); err != nil {
		return nil, err
	}

	if err := r.createRBAC(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create RBAC")
		return nil, err
	}

	if err := r.createServiceAccounts(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Service Accounts")
		return nil, err
	}

	if err := r.configureAnyUIDSCCs(metaObject); err != nil {
		logging.Log.Error(err, "Failed to configure SCCs")
		return nil, err
	}

	// Redis
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		if err := r.createRedisService(metaObject); err != nil {
			logging.Log.Error(err, "Failed to create Redis service")
			return nil, err
		}

		redisDeploymentResult, err := r.redisDeployment(metaObject)
		if err != nil {
			logging.Log.Error(err, "Failed to create Redis deployment")
			return redisDeploymentResult, err
		}

	}

	// Database (PostgreSQL/MySQL)
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server) {

		createDatabaseResult, err := r.createQuayDatabase(metaObject)

		if err != nil {
			logging.Log.Error(err, "Failed to create Quay database")
			return nil, err
		}

		if createDatabaseResult != nil {
			return createDatabaseResult, nil
		}

		err = r.configurePostgreSQL(metaObject)

		if err != nil {
			logging.Log.Error(err, "Failed to Setup Postgresql")
			return nil, err
		}

	}

	// Quay Resources
	if err := r.createQuayService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay service")
		return nil, err
	}

	if err := r.createQuayConfigService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config service")
		return nil, err
	}

	if err := r.createQuayRoute(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay route")
		return nil, err
	}

	if err := r.createQuayConfigRoute(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config route")
		return nil, err
	}

	if r.quayConfiguration.QuayRegistryIsProvisionPVCVolume {

		if err := r.quayRegistryStorage(metaObject); err != nil {
			logging.Log.Error(err, "Failed to create registry storage")
			return nil, err
		}

	} else {
		logging.Log.Info("Should attempt to remove")
	}

	return nil, nil
}

// DeployQuayConfiguration takes care of the deployment of the quay configuration
func (r *ReconcileQuayEcosystemConfiguration) DeployQuayConfiguration(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayConfigDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// DeployQuay takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) DeployQuay(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// DeployQuay takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) RemoveQuayConfigResources(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	quayName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.AppsV1().Deployments(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Deployment", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	// OpenShift Route
	route := &routev1.Route{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: quayName, Namespace: r.quayConfiguration.QuayEcosystem.Namespace}, route)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Finding Quay Config Route", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	err = r.reconcilerBase.GetClient().Delete(context.TODO(), route)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Failed to Delete Quay Config Route", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	err = r.k8sclient.CoreV1().Services(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Service", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayDatabase(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	// Update Metadata
	meta = resources.UpdateMetaWithName(meta, resources.GetQuayDatabaseName(r.quayConfiguration.QuayEcosystem))
	resources.BuildQuayDatabaseResourceLabels(meta.Labels)

	var databaseResources []metav1.Object

	if !r.quayConfiguration.ValidProvidedQuayDatabaseSecret {
		quayDatabaseSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetQuayDatabaseName(r.quayConfiguration.QuayEcosystem), meta, constants.DefaultQuayDatabaseCredentials)
		databaseResources = append(databaseResources, quayDatabaseSecret)

		r.quayConfiguration.QuayDatabase.Username = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsUsernameKey]
		r.quayConfiguration.QuayDatabase.Password = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsPasswordKey]
		r.quayConfiguration.QuayDatabase.Database = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsDatabaseKey]

	}

	// Create PVC
	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize) {
		databasePvc := resources.GetDatabasePVCDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize)
		databaseResources = append(databaseResources, databasePvc)
	}

	service := resources.GetDatabaseServiceResourceDefinition(meta, constants.PostgreSQLPort)
	databaseResources = append(databaseResources, service)

	deployment := resources.GetDatabaseDeploymentDefinition(meta, r.quayConfiguration)
	databaseResources = append(databaseResources, deployment)

	for _, resource := range databaseResources {
		err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resource)
		if err != nil {
			logging.Log.Error(err, "Error applying Quay database Resource")
			return nil, err
		}
	}

	// Verify Deployment
	deploymentName := meta.Name

	time.Sleep(time.Duration(2) * time.Second)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.Namespace)

}

func (r *ReconcileQuayEcosystemConfiguration) configurePostgreSQL(meta metav1.ObjectMeta) error {

	postgresqlPods := &corev1.PodList{}
	opts := &client.ListOptions{}
	opts.SetLabelSelector(fmt.Sprintf("%s=%s", constants.LabelCompoentKey, constants.LabelComponentQuayDatabaseValue))
	opts.InNamespace(r.quayConfiguration.QuayEcosystem.Namespace)

	err := r.reconcilerBase.GetClient().List(context.TODO(), opts, postgresqlPods)

	if err != nil {
		return err
	}

	postgresqlPodsItems := postgresqlPods.Items
	var podName string

	if len(postgresqlPodsItems) == 0 {
		return fmt.Errorf("Failed to locate any active PostgreSQL Pod")
	}

	podName = postgresqlPodsItems[0].Name

	success, stdout, stderr := k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"SELECT * FROM pg_available_extensions\" | /opt/rh/rh-postgresql96/root/usr/bin/psql -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to Exec into Postgresql Pod: %s", stderr)
	}

	if strings.Contains(stdout, "pg_trim") {
		return nil
	}

	success, stdout, stderr = k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"CREATE EXTENSION pg_trgm\" | /opt/rh/rh-postgresql96/root/usr/bin/psql -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to add pg_trim extension: %s", stderr)
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetConfigMapSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetSecretDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) configureAnyUIDSCCs(meta metav1.ObjectMeta) error {
	// Configure Redis Service Account for AnyUID SCC
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		err := r.configureAnyUIDSCC(constants.RedisServiceAccount, meta)

		if err != nil {
			return err
		}
	}

	// Configure Quay Service Account for AnyUID SCC
	err := r.configureAnyUIDSCC(constants.QuayServiceAccount, meta)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createServiceAccounts(meta metav1.ObjectMeta) error {
	// Create Redis Service Account
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		err := r.createServiceAccount(constants.RedisServiceAccount, meta)

		if err != nil {
			return err
		}
	}

	// Create Quay Service Account
	err := r.createServiceAccount(constants.QuayServiceAccount, meta)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createServiceAccount(serviceAccountName string, meta metav1.ObjectMeta) error {

	meta.Name = serviceAccountName

	found := &corev1.ServiceAccount{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: serviceAccountName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetServiceAccountDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRBAC(meta metav1.ObjectMeta) error {

	role := resources.GetRoleDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, role)
	if err != nil {
		return err
	}

	roleBinding := resources.GetRoleBindingDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, roleBinding)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayService(meta metav1.ObjectMeta) error {

	service := resources.GetQuayServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigService(meta metav1.ObjectMeta) error {

	service := resources.GetQuayConfigServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayRoute(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)

	route := resources.GetQuayRouteDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	createdRoute := &routev1.Route{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: meta.Name, Namespace: r.quayConfiguration.QuayEcosystem.Namespace}, createdRoute)

	if err != nil {
		return err
	}

	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.RouteHost) {
		r.quayConfiguration.QuayHostname = createdRoute.Spec.Host
	} else {
		r.quayConfiguration.QuayHostname = r.quayConfiguration.QuayEcosystem.Spec.Quay.RouteHost
	}

	r.quayConfiguration.QuayEcosystem.Status.Hostname = r.quayConfiguration.QuayHostname

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigRoute(meta metav1.ObjectMeta) error {

	route := resources.GetQuayConfigRouteDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) configureAnyUIDSCC(serviceAccountName string, meta metav1.ObjectMeta) error {

	sccUser := "system:serviceaccount:" + meta.Namespace + ":" + serviceAccountName

	anyUIDSCC := &ossecurityv1.SecurityContextConstraints{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: constants.AnyUIDSCC, Namespace: ""}, anyUIDSCC)

	if err != nil {
		logging.Log.Error(err, "Error occurred retrieving SCC")
		return err
	}

	sccUserFound := false
	for _, user := range anyUIDSCC.Users {
		if user == sccUser {

			sccUserFound = true
			break
		}
	}

	if !sccUserFound {
		anyUIDSCC.Users = append(anyUIDSCC.Users, sccUser)
		err = r.reconcilerBase.CreateOrUpdateResource(nil, r.quayConfiguration.QuayEcosystem.Namespace, anyUIDSCC)
		if err != nil {
			return err
		}
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayRegistryStorage(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayRegistryStorageName(r.quayConfiguration.QuayEcosystem)

	registryStoragePVC := resources.GetQuayPVCRegistryStorageDefinition(meta, r.quayConfiguration.QuayRegistryPersistentVolumeAccessModes, r.quayConfiguration.QuayRegistryPersistentVolumeSize, &r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.Local.PersistentVolumeStorageClassName)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, registryStoragePVC)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) removeQuayRegistryStorage(meta metav1.ObjectMeta) error {

	registryPVC := resources.GetQuayRegistryStorageName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.CoreV1().PersistentVolumeClaims(r.quayConfiguration.QuayEcosystem.Namespace).Delete(registryPVC, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Registry PVC", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", registryPVC)
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayDeployment(meta metav1.ObjectMeta) error {

	quayDeployment := resources.GetQuayDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayConfigDeployment(meta metav1.ObjectMeta) error {

	if !r.quayConfiguration.ValidProvidedQuayConfigPasswordSecret {
		quayConfigSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem), meta, constants.DefaultQuayConfigCredentials)

		err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayConfigSecret)

		if err != nil {
			return err
		}

	}

	quayDeployment := resources.GetQuayConfigDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRedisService(meta metav1.ObjectMeta) error {

	service := resources.GetRedisServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) redisDeployment(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	redisDeployment := resources.GetRedisDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, redisDeployment)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	redisDeploymentName := resources.GetRedisResourcesName(r.quayConfiguration.QuayEcosystem)
	return r.verifyDeployment(redisDeploymentName, r.quayConfiguration.QuayEcosystem.Namespace)
}

// Verify Deployment
func (r *ReconcileQuayEcosystemConfiguration) verifyDeployment(deploymentName string, deploymentNamespace string) (*reconcile.Result, error) {

	deployment := &appsv1.Deployment{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: deploymentName, Namespace: deploymentNamespace}, deployment)

	if err != nil {
		return nil, err
	}

	if deployment.Status.AvailableReplicas != 1 {
		scaled := k8sutils.GetDeploymentStatus(r.k8sclient, deploymentNamespace, deploymentName)

		if !scaled {
			return &reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
		}

	}

	return nil, nil

}
