package provisioning

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	ossecurityv1 "github.com/openshift/api/security/v1"
	qclient "github.com/redhat-cop/quay-operator/pkg/client"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/externalaccess"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"github.com/redhat-cop/quay-operator/pkg/k8sutils"
	yaml "gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-cop/operator-utils/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
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

// CoreQuayResourceDeployment takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) CoreQuayResourceDeployment(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

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

	// Configure SCC when running in OpenShift
	if r.quayConfiguration.IsOpenShift {
		if err := r.configureAnyUIDSCCs(metaObject); err != nil {
			logging.Log.Error(err, "Failed to configure SCCs")
			return nil, err
		}
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

	// Quay Database (PostgreSQL/MySQL)
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
			logging.Log.Error(err, "Failed to Quay Setup Postgresql")
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

	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) {

		if err := r.quayRegistryStorage(metaObject); err != nil {
			logging.Log.Error(err, "Failed to create registry storage")
			return nil, err
		}

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

// DeployQuayRepoMirror takes care of the deployment of the quay repo mirror
func (r *ReconcileQuayEcosystemConfiguration) DeployQuayRepoMirror(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayRepoMirrorDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Repo Mirror deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetQuayRepoMirrorResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// DeployQuay takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) DeployQuay(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay deployment")
		return nil, err
	}

	if !r.quayConfiguration.QuayEcosystem.Spec.Quay.SkipSetup {

		time.Sleep(time.Duration(2) * time.Second)

		// Verify Deployment
		deploymentName := resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)

		return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

	}

	logging.Log.Info("Skipping Quay Deployment verification as setup marked as skipped")
	return &reconcile.Result{}, nil

}

// DeployClair takes care of the deployment of clair
func (r *ReconcileQuayEcosystemConfiguration) DeployClair(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.clairDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Clair deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetClairResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// RemoveQuayConfigResources removes the resources associated with the Quay Configuration
func (r *ReconcileQuayEcosystemConfiguration) RemoveQuayConfigResources(metaObject metav1.ObjectMeta, external externalaccess.ExternalAccess) (*reconcile.Result, error) {

	quayName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.AppsV1().Deployments(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Deployment", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	// OpenShift External Access
	if err := external.RemoveQuayConfigExternalAccess(metaObject); err != nil {
		logging.Log.Error(err, "Error Deleting Quay Config Deployment", "Deployment", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	err = r.k8sclient.CoreV1().Services(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Service", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	return nil, nil
}

// ConfigureSecurityScanner handles the steps for configuring the security scanner
func (r *ReconcileQuayEcosystemConfiguration) ConfigureSecurityScanner(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	scannerSecretName := resources.GetSecurityScannerSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = scannerSecretName

	scannerSecret := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: scannerSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, scannerSecret)

	// Check to see if the secret exists
	if err == nil {

		// Set so that it can be injected into Clair ConfigMap
		r.quayConfiguration.SecurityScannerKeyID = string(scannerSecret.Data[constants.SecurityScannerServiceSecretKIDKey])
		return nil, nil
	}

	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	httpClient := resources.GetDefaultHTTPClient()

	quayConfigURL := fmt.Sprintf("https://%s", r.quayConfiguration.QuayConfigHostname)

	configClient := qclient.NewClient(httpClient, quayConfigURL, r.quayConfiguration.QuayConfigUsername, r.quayConfiguration.QuayConfigPassword)

	_, configuredKeys, err := configClient.GetKeys()

	if err != nil {
		logging.Log.Error(err, "Error obtaining service keys")
		return nil, err
	}

	// Check if a valid service key is present
	var serviceKeyFound = false
	for _, val := range configuredKeys.Keys {
		if val.Service == constants.SecurityScannerService {
			serviceKeyFound = true
		}
	}

	// Create a new service key and secret
	if !serviceKeyFound {
		_, keyCreationResponse, err := configClient.CreateKey(qclient.KeyCreationRequest{
			Name:    constants.SecurityScannerService,
			Service: constants.SecurityScannerService,
			Notes:   resources.GetSecurityScannerKeyNotes(r.quayConfiguration.QuayEcosystem),
		})

		if err != nil {
			return nil, err
		}

		r.quayConfiguration.SecurityScannerKeyID = keyCreationResponse.KID

		// Create new Secret from returned KID
		scannerSecret = resources.GetSecretDefinition(meta)

		scannerSecret.StringData = map[string]string{
			constants.SecurityScannerServiceSecretKey:    keyCreationResponse.PrivateKey,
			constants.SecurityScannerServiceSecretKIDKey: keyCreationResponse.KID,
		}

		meta.Name = scannerSecretName

		err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, scannerSecret)

		if err != nil {
			return nil, err
		}

	}

	return nil, nil

}

// ManageClairComponents contains the logic to manage the majority of the components necessary to enable security scanning using Clair
func (r *ReconcileQuayEcosystemConfiguration) ManageClairComponents(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	coreClairResourceDeploymentResult, err := r.coreClairResourceDeployment(metaObject)

	if err != nil {
		logging.Log.Error(err, "Failed to configure core clair resources")
		return nil, err
	}

	if coreClairResourceDeploymentResult != nil {
		return coreClairResourceDeploymentResult, nil
	}

	return nil, nil

}

// coreClairResourceDeployment manages the core Clair resources including the database
func (r *ReconcileQuayEcosystemConfiguration) coreClairResourceDeployment(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.createClairService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Clair service")
		return nil, err
	}

	if err := r.manageClairConfigMap(metaObject); err != nil {
		logging.Log.Error(err, "Failed to manage Clair ConfigMap")
		return nil, err
	}

	// Database (PostgreSQL/MySQL)
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Clair.Database.Server) {

		createClairDatabaseResult, err := r.createClairDatabase(metaObject)

		if err != nil {
			logging.Log.Error(err, "Failed to create Clair database")
			return nil, err
		}

		if createClairDatabaseResult != nil {
			return createClairDatabaseResult, nil
		}

	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayDatabase(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	// Update Metadata
	meta = resources.UpdateMetaWithName(meta, resources.GetDatabaseResourceName(r.quayConfiguration.QuayEcosystem, constants.DatabaseComponentQuay))
	resources.BuildQuayDatabaseResourceLabels(meta.Labels)

	var databaseResources []metav1.Object

	if !r.quayConfiguration.ValidProvidedQuayDatabaseSecret {
		quayDatabaseSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetDatabaseResourceName(r.quayConfiguration.QuayEcosystem, constants.DatabaseComponentQuay), meta, constants.DefaultQuayDatabaseCredentials)
		databaseResources = append(databaseResources, quayDatabaseSecret)
	}

	// Create PVC
	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize) {
		databasePvc := resources.GetDatabasePVCDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize)
		databaseResources = append(databaseResources, databasePvc)
	}

	service := resources.GetDatabaseServiceResourceDefinition(meta, constants.PostgreSQLPort)
	databaseResources = append(databaseResources, service)

	deployment := resources.GetDatabaseDeploymentDefinition(meta, r.quayConfiguration, r.quayConfiguration.QuayEcosystem.Spec.Quay.Database, constants.DatabaseComponentQuay)
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

// TODO: Consolidate into a single create Database for both Clair and Quay
func (r *ReconcileQuayEcosystemConfiguration) createClairDatabase(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	// Update Metadata
	meta = resources.UpdateMetaWithName(meta, resources.GetDatabaseResourceName(r.quayConfiguration.QuayEcosystem, constants.DatabaseComponentClair))
	resources.BuildClairDatabaseResourceLabels(meta.Labels)

	var databaseResources []metav1.Object

	if !r.quayConfiguration.ValidProvidedClairDatabaseSecret {
		clairDatabaseSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetDatabaseResourceName(r.quayConfiguration.QuayEcosystem, constants.DatabaseComponentClair), meta, constants.DefaultClairDatabaseCredentials)
		databaseResources = append(databaseResources, clairDatabaseSecret)
	}

	// Create PVC
	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Clair.Database.VolumeSize) {
		databasePvc := resources.GetDatabasePVCDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Clair.Database.VolumeSize)
		databaseResources = append(databaseResources, databasePvc)
	}

	service := resources.GetDatabaseServiceResourceDefinition(meta, constants.PostgreSQLPort)
	databaseResources = append(databaseResources, service)

	deployment := resources.GetDatabaseDeploymentDefinition(meta, r.quayConfiguration, r.quayConfiguration.QuayEcosystem.Spec.Clair.Database, constants.DatabaseComponentClair)
	databaseResources = append(databaseResources, deployment)

	for _, resource := range databaseResources {
		err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resource)
		if err != nil {
			logging.Log.Error(err, "Error applying Clair database Resource")
			return nil, err
		}
	}

	// Verify Deployment
	deploymentName := meta.Name

	time.Sleep(time.Duration(2) * time.Second)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.Namespace)

}

func (r *ReconcileQuayEcosystemConfiguration) configurePostgreSQL(meta metav1.ObjectMeta) error {

	postgresqlPods := corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(r.quayConfiguration.QuayEcosystem.Namespace),
		client.MatchingLabels(map[string]string{constants.LabelCompoentKey: constants.LabelComponentQuayDatabaseValue}),
	}

	err := r.reconcilerBase.GetClient().List(context.TODO(), &postgresqlPods, opts...)

	if err != nil {
		return err
	}

	postgresqlPodsItems := postgresqlPods.Items
	var podName string

	if len(postgresqlPodsItems) == 0 {
		return fmt.Errorf("Failed to locate any active PostgreSQL Pod")
	}

	podName = postgresqlPodsItems[0].Name

	success, stdout, stderr := k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"SELECT * FROM pg_available_extensions\" | $(which psql) -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to Exec into Postgresql Pod: %s", stderr)
	}

	if strings.Contains(stdout, "pg_trim") {
		return nil
	}

	success, stdout, stderr = k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"CREATE EXTENSION pg_trgm\" | $(which psql) -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to add pg_trim extension: %s", stderr)
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetQuayConfigMapSecretName(r.quayConfiguration.QuayEcosystem)

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

	// Configure Quay Service Account for AnyUID SCC
	for _, serviceAccountName := range constants.RequiredAnyUIDSccServiceAccounts {
		err := r.configureAnyUIDSCC(serviceAccountName, meta)

		if err != nil {
			return err
		}
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

	// Create Clair Service Account
	if r.quayConfiguration.QuayEcosystem.Spec.Clair != nil && r.quayConfiguration.QuayEcosystem.Spec.Clair.Enabled {

		err = r.createServiceAccount(constants.ClairServiceAccount, meta)

		if err != nil {
			return err
		}

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

	serviceName := resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)
	service := resources.GetQuayServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	return r.manageService(serviceName, service)

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigService(meta metav1.ObjectMeta) error {

	serviceName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)
	service := resources.GetQuayConfigServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	return r.manageService(serviceName, service)

}

func (r *ReconcileQuayEcosystemConfiguration) createClairService(meta metav1.ObjectMeta) error {

	serviceName := resources.GetClairResourcesName(r.quayConfiguration.QuayEcosystem)
	service := resources.GetClairServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	return r.manageService(serviceName, service)

}

func (r *ReconcileQuayEcosystemConfiguration) manageClairConfigMap(meta metav1.ObjectMeta) error {

	clairConfigMapName := resources.GetClairConfigMapName(r.quayConfiguration.QuayEcosystem)

	meta.Name = clairConfigMapName

	clairConfigMap := &corev1.ConfigMap{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: clairConfigMapName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, clairConfigMap)

	if err != nil && apierrors.IsNotFound(err) {

		clairConfigMap = resources.GetConfigMapDefinition(meta)

		err = r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, clairConfigMap)

		if err != nil {
			return err
		}

		time.Sleep(time.Duration(2) * time.Second)

		// Get fresh copy of the Object
		err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: clairConfigMapName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, clairConfigMap)

		if err != nil {
			return err
		}

	} else if err != nil {
		logging.Log.Info("Error Occurred while retrieving ConfigMap")
		return err
	}

	var clairConfigFile qclient.ClairFile

	//	if _, configFound := clairConfigMap.Data[constants.ClairConfigFileKey]; configFound {

	//err = yaml.Unmarshal([]byte(configVal), &clairConfigFile)

	//if err != nil {
	//	return err
	//}

	//	} else {
	clairConfigFile = resources.GenerateDefaultClairConfigFile()
	//	}

	clairConfigFile.Clair.Database.Options["source"] = fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", r.quayConfiguration.ClairDatabase.Username, r.quayConfiguration.ClairDatabase.Password, r.quayConfiguration.ClairDatabase.Server, r.quayConfiguration.ClairDatabase.Database)

	clairAudience, _ := url.Parse(resources.GetClairEndpointAddress(r.quayConfiguration.QuayEcosystem))

	clairConfigFile.Clair.Updater.Interval = r.quayConfiguration.ClairUpdateInterval

	clairConfigFile.Clair.Notifier.Params["http"] = &qclient.ClairHttpNotifier{
		Endpoint: fmt.Sprintf("https://%s/secscan/notify", r.quayConfiguration.QuayEcosystem.Status.Hostname),
		Proxy:    "http://localhost:6063",
	}

	clairConfigFile.JwtProxy.VerifierProxies[0].Verifier.KeyServer.Options = map[string]interface{}{
		"registry": fmt.Sprintf("https://%s/keys/", r.quayConfiguration.QuayEcosystem.Status.Hostname),
	}

	clairConfigFile.JwtProxy.VerifierProxies[0].Verifier.Audience = qclient.URL{
		URL: clairAudience,
	}

	clairConfigFile.JwtProxy.SignerProxy.Signer.PrivateKey.Options["key_id"] = r.quayConfiguration.SecurityScannerKeyID
	clairConfigFile.JwtProxy.SignerProxy.Signer.PrivateKey.Options["private_key_path"] = constants.ClairSecurityScannerPath

	marshaledConfigFile, err := yaml.Marshal(clairConfigFile)
	if err != nil {
		return err
	}

	if clairConfigMap.Data == nil {
		clairConfigMap.Data = map[string]string{}
	}

	clairConfigMap.Data[constants.ClairConfigFileKey] = string(marshaledConfigFile)

	err = r.reconcilerBase.CreateOrUpdateResource(nil, r.quayConfiguration.QuayEcosystem.Namespace, clairConfigMap)
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

	for _, registryBackend := range r.quayConfiguration.RegistryBackends {

		if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Local) {
			registryVolumeName := resources.GetRegistryStorageVolumeName(r.quayConfiguration.QuayEcosystem, registryBackend.Name)

			meta.Name = registryVolumeName

			registryStoragePVC := resources.GetQuayPVCRegistryStorageDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeAccessModes, r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeSize, &r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeStorageClassName)

			err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, registryStoragePVC)

			if err != nil {
				return err
			}

		}

	}

	return nil

}

// removeQuayRegistryStorage handles removing persistent storage for local storage
func (r *ReconcileQuayEcosystemConfiguration) removeQuayRegistryStorage(meta metav1.ObjectMeta) error {

	registryPVC := resources.GetQuayRegistryStorageName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.CoreV1().PersistentVolumeClaims(r.quayConfiguration.QuayEcosystem.Namespace).Delete(registryPVC, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Registry PVC", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", registryPVC)
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) ManageQuayEcosystemCertificates(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	configSecretName := resources.GetQuayConfigMapSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	appConfigSecret := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, appConfigSecret)

	if err != nil {

		if apierrors.IsNotFound(err) {
			// Config Secret Not Found. Requeue object
			return &reconcile.Result{}, nil
		}
		return nil, err
	}

	if !isQuayCertificatesConfigured(appConfigSecret) {

		if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.SslCertificatesSecretName) {
			var certBytes, privKeyBytes []byte

			// Check if hostname is a IP address or hostname
			hostnameParts := strings.Split(r.quayConfiguration.QuayHostname, ":")

			parsedIP := net.ParseIP(hostnameParts[0])

			if parsedIP == nil {
				certBytes, privKeyBytes, err = cert.GenerateSelfSignedCertKey(constants.QuayEnterprise, []net.IP{}, []string{hostnameParts[0]})
			} else {
				certBytes, privKeyBytes, err = cert.GenerateSelfSignedCertKey(constants.QuayEnterprise, []net.IP{parsedIP}, []string{})
			}

			if err != nil {
				logging.Log.Error(err, "Error creating public/private key")
				return nil, err
			}

			r.quayConfiguration.QuaySslCertificate = certBytes
			r.quayConfiguration.QuaySslPrivateKey = privKeyBytes

		}
	} else {
		if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.SslCertificatesSecretName) {
			r.quayConfiguration.QuaySslPrivateKey = appConfigSecret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]
			r.quayConfiguration.QuaySslCertificate = appConfigSecret.Data[constants.QuayAppConfigSSLCertificateSecretKey]
		}

	}

	if appConfigSecret.Data == nil {
		appConfigSecret.Data = map[string][]byte{}
	}

	appConfigSecret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey] = r.quayConfiguration.QuaySslPrivateKey
	appConfigSecret.Data[constants.QuayAppConfigSSLCertificateSecretKey] = r.quayConfiguration.QuaySslCertificate

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, appConfigSecret)

	if err != nil {
		logging.Log.Error(err, "Error Updating app secret with certificates")
		return nil, err
	}

	// Manage Clair Certificates
	if r.quayConfiguration.QuayEcosystem.Spec.Clair != nil && r.quayConfiguration.QuayEcosystem.Spec.Clair.Enabled {
		clairSslSecretName := resources.GetClairSSLSecretName(r.quayConfiguration.QuayEcosystem)

		meta.Name = clairSslSecretName

		clairSslSecret := &corev1.Secret{}
		err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: clairSslSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, clairSslSecret)

		if err != nil && !apierrors.IsNotFound(err) {
			logging.Log.Error(err, "Error Finding Clair SSL Secret", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", clairSslSecretName)
			return nil, err
		}

		// Only Process if Secret is not found
		if apierrors.IsNotFound(err) {

			clairServiceName := resources.GetClairResourcesName(r.quayConfiguration.QuayEcosystem)
			certBytes, privKeyBytes, err := cert.GenerateSelfSignedCertKey(clairServiceName, []net.IP{}, resources.GenerateClairCertificateSANs(clairServiceName, r.quayConfiguration.QuayEcosystem.Namespace))
			if err != nil {
				logging.Log.Error(err, "Error creating public/private key")
				return nil, err
			}

			clairSslSecret = resources.GetTLSSecretDefinition(meta, privKeyBytes, certBytes)

			err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, clairSslSecret)

			if err != nil {
				logging.Log.Error(err, "Error creating Clair SSL secret")
				return nil, err
			}

		}

	}

	return nil, nil
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

func (r *ReconcileQuayEcosystemConfiguration) quayRepoMirrorDeployment(meta metav1.ObjectMeta) error {

	quayDeployment := resources.GetQuayRepoMirrorDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) clairDeployment(meta metav1.ObjectMeta) error {

	clairDeployment := resources.GetClairDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, clairDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRedisService(meta metav1.ObjectMeta) error {

	serviceName := resources.GetRedisResourcesName(r.quayConfiguration.QuayEcosystem)
	service := resources.GetRedisServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	return r.manageService(serviceName, service)

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

func (r *ReconcileQuayEcosystemConfiguration) manageService(serviceName string, service *corev1.Service) error {

	existingService := &corev1.Service{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem), Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, existingService)

	if err != nil {
		if apierrors.IsNotFound(err) {
			err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil

}

func isQuayCertificatesConfigured(secret *corev1.Secret) bool {

	if !utils.IsZeroOfUnderlyingType(secret) {
		if _, found := secret.Data[constants.QuayAppConfigSSLCertificateSecretKey]; !found {
			return false
		}

		if len(secret.Data[constants.QuayAppConfigSSLCertificateSecretKey]) == 0 {
			return false
		}

		if _, found := secret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]; !found {
			return false
		}

		if len(secret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]) == 0 {
			return false
		}

		return true

	}
	return false
}
