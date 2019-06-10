package validation

import (
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetDefaults(client client.Client, quayConfiguration *resources.QuayConfiguration) bool {

	changed := false

	// Initialize Base Variables
	quayConfiguration.QuayConfigUsername = constants.QuayConfigUsername
	quayConfiguration.QuayConfigPassword = constants.QuayConfigDefaultPasswordValue
	quayConfiguration.QuaySuperuserUsername = constants.QuaySuperuserDefaultUsername
	quayConfiguration.QuaySuperuserPassword = constants.QuaySuperuserDefaultPassword
	quayConfiguration.QuaySuperuserEmail = constants.QuaySuperuserDefaultEmail
	quayConfiguration.QuayConfigPasswordSecret = resources.GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)

	// Core Quay Values
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Image) {
		changed = true
		quayConfiguration.QuayEcosystem.Spec.Quay.Image = constants.QuayImage
	}
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Image) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Redis.Image = constants.RedisImage
		}

	}

	// User would like to have a database automatically provisioned if server not provided
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database) || utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server) {

		// If a user does not provide a server, one needs to be provisoned
		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server) {

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image = constants.PostgresqlImage
			}
		}

	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment) && quayConfiguration.QuayEcosystem.Spec.Quay.KeepConfigDeployment {
		quayConfiguration.DeployQuayConfiguration = true
	}

	if !quayConfiguration.QuayEcosystem.Status.SetupComplete {
		quayConfiguration.DeployQuayConfiguration = true
	}

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) || !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.RegistryStorageType.Local) {

		// Check if we want to provision a PVC to back the registry
		if !quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.RegistryStorageType.Local.Ephemeral {
			quayConfiguration.QuayRegistryIsProvisionPVCVolume = true

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.Local.PersistentVolumeAccessModes) {
				quayConfiguration.QuayRegistryPersistentVolumeAccessModes = constants.QuayRegistryStoragePersistentVolumeAccessModes
			} else {
				quayConfiguration.QuayRegistryPersistentVolumeAccessModes = quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.RegistryStorageType.Local.PersistentVolumeAccessModes
			}

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.RegistryStorageType.Local.PersistentVolumeSize) {
				quayConfiguration.QuayRegistryPersistentVolumeSize = constants.QuayRegistryStoragePersistentVolumeStoreSize
			}
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.RegistryStorageType.Local.StorageDirectory) {
			quayConfiguration.QuayRegistryStorageDirectory = constants.QuayRegistryStorageDirectory
		}

	}

	return changed
}
