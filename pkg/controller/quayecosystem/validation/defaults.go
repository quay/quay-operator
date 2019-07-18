package validation

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
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

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) {

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeAccessModes) {
			quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeAccessModes = constants.QuayRegistryStoragePersistentVolumeAccessModes
			changed = true
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeSize) {
			quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeSize = constants.QuayRegistryStoragePersistentVolumeStoreSize
			changed = true
		}
	}

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends) {
		// Generate Default Local Storage
		quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends = []redhatcopv1alpha1.RegistryBackend{
			redhatcopv1alpha1.RegistryBackend{
				Name: constants.RegistryStorageDefaultName,
				RegistryBackendSource: redhatcopv1alpha1.RegistryBackendSource{
					Local: &redhatcopv1alpha1.LocalRegistryBackendSource{
						StoragePath: constants.QuayRegistryStoragePath,
					},
				},
			},
		}

		changed = true
	}

	return changed
}
