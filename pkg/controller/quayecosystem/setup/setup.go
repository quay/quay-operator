package setup

import (
	"fmt"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/client"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"k8s.io/client-go/kubernetes"
)

// Registry Represents the status returned from the Quay Registry
type RegistryStatus string

var (
	RegistryStatusConfigDB RegistryStatus = "config-db"
	RegistryStatusSetupDB  RegistryStatus = "setup-db"
	RegistryStatusConfig   RegistryStatus = "config"
)

type QuaySetupManager struct {
	reconcilerBase util.ReconcilerBase
	k8sclient      kubernetes.Interface
}

type QuaySetupInstance struct {
	quayConfiguration resources.QuayConfiguration
	setupClient       client.QuayClient
}

func NewQuaySetupManager(reconcilerBase util.ReconcilerBase, k8sclient kubernetes.Interface) *QuaySetupManager {
	return &QuaySetupManager{reconcilerBase: reconcilerBase, k8sclient: k8sclient}
}

func (*QuaySetupManager) NewQuaySetupInstance(quayConfiguration *resources.QuayConfiguration) (*QuaySetupInstance, error) {

	httpClient := resources.GetDefaultHTTPClient()

	quayConfigURL := fmt.Sprintf("https://%s", quayConfiguration.QuayConfigHostname)

	setupClient := client.NewClient(httpClient, quayConfigURL, quayConfiguration.QuayConfigUsername, quayConfiguration.QuayConfigPassword)

	quaySetupInstance := QuaySetupInstance{
		quayConfiguration: *quayConfiguration,
		setupClient:       *setupClient,
	}

	return &quaySetupInstance, nil
}

// SetupQuay performs the initialization and initial configuration of the Quay server
func (qm *QuaySetupManager) SetupQuay(quaySetupInstance *QuaySetupInstance) error {

	_, _, err := quaySetupInstance.setupClient.GetRegistryStatus()

	if err != nil {
		logging.Log.Error(err, "Failed to obtain initial registry status")
		return err
	}

	_, _, err = quaySetupInstance.setupClient.InitializationConfiguration()

	if err != nil {
		logging.Log.Error(err, "Failed to Initialize")
		return err
	}

	quayConfig := client.QuayConfig{
		Config: map[string]interface{}{},
	}

	quayConfig.Config["DB_URI"] = fmt.Sprintf("postgresql://%s:%s@%s/%s", quaySetupInstance.quayConfiguration.QuayDatabase.Username, quaySetupInstance.quayConfiguration.QuayDatabase.Password, quaySetupInstance.quayConfiguration.QuayDatabase.Server, quaySetupInstance.quayConfiguration.QuayDatabase.Database)

	err = qm.validateComponent(quaySetupInstance, quayConfig, client.DatabaseValidation)

	if err != nil {
		return err
	}

	redisConfiguration := map[string]interface{}{
		"host": quaySetupInstance.quayConfiguration.RedisHostname,
	}

	if !utils.IsZeroOfUnderlyingType(quaySetupInstance.quayConfiguration.RedisPort) {
		redisConfiguration["port"] = quaySetupInstance.quayConfiguration.RedisPort
	}

	if !utils.IsZeroOfUnderlyingType(quaySetupInstance.quayConfiguration.RedisPassword) {
		redisConfiguration["password"] = quaySetupInstance.quayConfiguration.RedisPassword
	}

	quayConfig.Config["BUILDLOGS_REDIS"] = redisConfiguration
	quayConfig.Config["USER_EVENTS_REDIS"] = redisConfiguration
	quayConfig.Config["SERVER_HOSTNAME"] = quaySetupInstance.quayConfiguration.QuayHostname
	quayConfig.Config["FEATURE_REPO_MIRROR"] = quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.EnableRepoMirroring
	quayConfig.Config["REPO_MIRROR_TLS_VERIFY"] = quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.RepoMirrorTLSVerify

	if !utils.IsZeroOfUnderlyingType(quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.RepoMirrorServerHostname) {
		quayConfig.Config["REPO_MIRROR_SERVER_HOSTNAME"] = quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.RepoMirrorServerHostname
	}

	_, _, err = quaySetupInstance.setupClient.UpdateQuayConfiguration(quayConfig)

	if err != nil {
		logging.Log.Error(err, "Failed to update quay configuration")
		return fmt.Errorf("Failed to update quay configuration: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.SetupDatabase()

	if err != nil {
		logging.Log.Error(err, "Failed to setup database")
		return fmt.Errorf("Failed to setup database: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.CreateSuperuser(client.QuayCreateSuperuserRequest{
		Username:        quaySetupInstance.quayConfiguration.QuaySuperuserUsername,
		Email:           quaySetupInstance.quayConfiguration.QuaySuperuserEmail,
		Password:        quaySetupInstance.quayConfiguration.QuaySuperuserPassword,
		ConfirmPassword: quaySetupInstance.quayConfiguration.QuaySuperuserPassword,
	})

	if err != nil {
		logging.Log.Error(err, "Failed to create superuser")
		return fmt.Errorf("Failed to create superuser: %s", err.Error())
	}

	_, quayConfig, err = quaySetupInstance.setupClient.GetQuayConfiguration()

	if err != nil {
		logging.Log.Error(err, "Failed to get Quay Configuration")
		return fmt.Errorf("Failed to get Quay Configuration: %s", err.Error())
	}

	// Setup Storage
	distributedStorageConfig := map[string][]interface{}{}
	distributedStoragePreference := []string{}
	distributedStorageReplicateByDefault := []string{}
	storageReplication := false

	for _, registryBackend := range quaySetupInstance.quayConfiguration.RegistryBackends {

		var quayRegistry []interface{}
		if registryBackend.ReplicateByDefault != nil && *registryBackend.ReplicateByDefault == true {
			distributedStorageReplicateByDefault = append(distributedStorageReplicateByDefault, registryBackend.Name)
			storageReplication = true
		}

		if quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.EnableStorageReplication {
			distributedStoragePreference = append(distributedStoragePreference, registryBackend.Name)
			storageReplication = true
		}

		if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Local) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeLocalStorageName)
			quayRegistry = append(quayRegistry, resources.LocalRegistryBackendToQuayLocalRegistryBackend(registryBackend.RegistryBackendSource.Local))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.S3) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeS3StorageName)
			quayRegistry = append(quayRegistry, resources.S3RegistryBackendToQuayS3RegistryBackend(registryBackend.RegistryBackendSource.S3))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.GoogleCloud) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeGoogleCloudStorageName)
			quayRegistry = append(quayRegistry, resources.GoogleCloudRegistryBackendToQuayGoogleCloudRegistryBackend(registryBackend.RegistryBackendSource.GoogleCloud))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Azure) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeAzureStorageName)
			quayRegistry = append(quayRegistry, resources.AzureRegistryBackendToQuayAzureRegistryBackend(registryBackend.RegistryBackendSource.Azure))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.RHOCS) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeRHOCSStorageName)
			quayRegistry = append(quayRegistry, resources.RHOCSRegistryBackendToQuayRHOCSRegistryBackend(registryBackend.RegistryBackendSource.RHOCS))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.RADOS) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeRADOSStorageName)
			quayRegistry = append(quayRegistry, resources.RADOSRegistryBackendToQuayRADOSRegistryBackend(registryBackend.RegistryBackendSource.RADOS))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Swift) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeSwiftStorageName)
			quayRegistry = append(quayRegistry, resources.SwiftRegistryBackendToQuaySwiftRegistryBackend(registryBackend.RegistryBackendSource.Swift))
		} else if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.CloudfrontS3) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeCloudfrontS3StorageName)
			quayRegistry = append(quayRegistry, resources.CloudfrontS3RegistryBackendToQuayCloudfrontS3RegistryBackend(registryBackend.RegistryBackendSource.CloudfrontS3))
		}

		registryBackend.ReplicateByDefault = nil

		distributedStorageConfig[registryBackend.Name] = quayRegistry

	}

	quayConfig.Config["DISTRIBUTED_STORAGE_CONFIG"] = distributedStorageConfig
	quayConfig.Config["FEATURE_STORAGE_REPLICATION"] = storageReplication

	// Set storage preference if not set
	if len(quaySetupInstance.quayConfiguration.RegistryBackends) == 1 {
		distributedStoragePreference = append(distributedStoragePreference, quaySetupInstance.quayConfiguration.RegistryBackends[0].Name)
	}

	quayConfig.Config["DISTRIBUTED_STORAGE_PREFERENCE"] = distributedStoragePreference

	// Set default storage preference locations if defined
	if len(distributedStorageReplicateByDefault) > 0 {
		quayConfig.Config["DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS"] = distributedStorageReplicateByDefault
	}

	// Setup Security Scanner
	if quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Clair != nil && quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Clair.Enabled {
		quayConfig.Config["SECURITY_SCANNER_ISSUER_NAME"] = constants.SecurityScannerService
		quayConfig.Config["SECURITY_SCANNER_ENDPOINT"] = resources.GetClairEndpointAddress(quaySetupInstance.quayConfiguration.QuayEcosystem)
		quayConfig.Config["FEATURE_SECURITY_SCANNER"] = true
	}

	// Add Certificates
	_, _, err = quaySetupInstance.setupClient.UploadFileResource(constants.QuayAppConfigSSLPrivateKeySecretKey, quaySetupInstance.quayConfiguration.QuaySslPrivateKey)

	if err != nil {
		logging.Log.Error(err, "Failed to upload SSL certificates")
		return fmt.Errorf("Failed to upload SSL certificates: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.UploadFileResource(constants.QuayAppConfigSSLCertificateSecretKey, quaySetupInstance.quayConfiguration.QuaySslCertificate)

	if err != nil {
		return err
	}

	// Validate multiple components
	for _, validationComponent := range []client.QuayValidationType{client.RedisValidation, client.RegistryValidation, client.TimeMachineValidation, client.AccessValidation, client.SslValidation} {
		err = qm.validateComponent(quaySetupInstance, quayConfig, validationComponent)

		if err != nil {
			logging.Log.Error(err, "Failed to Validate Component")
			return fmt.Errorf("Failed to Validate Component: %s", err.Error())
		}
	}

	quayConfig.Config["PREFERRED_URL_SCHEME"] = "https"

	quayConfig.Config["SETUP_COMPLETE"] = true
	_, quayConfig, err = quaySetupInstance.setupClient.UpdateQuayConfiguration(quayConfig)

	if err != nil {
		logging.Log.Error(err, "Failed to update Quay Configuration")
		return fmt.Errorf("Failed to update Quay Configuration: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.CompleteSetup()

	if err != nil {
		logging.Log.Error(err, "Failed to complete Quay Configuration setup")
		return fmt.Errorf("Failed to complete Quay Configuration setup: %s", err.Error())
	}

	return nil

}

func (*QuaySetupManager) validateComponent(quaySetupInstance *QuaySetupInstance, quayConfig client.QuayConfig, validationType client.QuayValidationType) error {

	_, validateResponse, err := quaySetupInstance.setupClient.ValidateComponent(quayConfig, validationType)

	if err != nil {
		return err
	}

	if !validateResponse.Status {
		return fmt.Errorf("%s Validation Failed: %s", validationType, validateResponse.Reason)
	}

	return nil
}
