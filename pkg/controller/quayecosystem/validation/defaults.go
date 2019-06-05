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

	if checkEmptyOrNull(quayConfiguration.QuayEcosystem.Spec.Quay.IsOpenShift) || !quayConfiguration.QuayEcosystem.Spec.Quay.IsOpenShift {
		quayConfiguration.IsOpenShift = false
	} else {
		quayConfiguration.IsOpenShift = true
	}

	// Core Quay Values
	if checkEmptyOrNull(quayConfiguration.QuayEcosystem.Spec.Quay.Image) {
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

	/*
		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) {

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.StorageDirectory) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.StorageDirectory = constants.QuayRegistryStorageDirectory
			}

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.AccessModes) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.AccessModes = constants.QuayRegistryStoragePersistentVolumeAccessModes
			}

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.Capacity) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.Capacity = constants.QuayRegistryStoragePersistentVolumeStoreSize
			}

		}
	*/

	/* Things to add
	1. Superuser
	*/

	// if !quayConfiguration.QuayEcosystem.Status.ProvisioningComplete {
	/*
	  1. Superuser
	*/
	/*
		quayConfiguration.QuaySuperuserUsername = constants.QuaySuperuserDefaultUsername
		quayConfiguration.QuaySuperuserPassword = constants.QuaySuperuserDefaultPassword
		quayConfiguration.QuaySuperuserEmail = constants.QuaySuperuserDefaultEmail

		// }

		quayConfiguration.QuayConfigUsername = constants.QuayConfigUsername
		quayConfiguration.QuayConfigPassword = constants.QuayConfigDefaultPasswordValue

	*/
	return changed
}

// func GetDefaultDatabaseSecret(meta metav1.ObjectMeta, credentials map[string]string) *corev1.Secret {

// 	defaultSecret := &corev1.Secret{
// 		ObjectMeta: meta,
// 		StringData: map[string]string{
// 			constants.DatabaseCredentialsDatabaseKey: credentials[constants.DatabaseCredentialsDatabaseKey],
// 			constants.DatabaseCredentialsUsernameKey: credentials[constants.DatabaseCredentialsUsernameKey],
// 			constants.DatabaseCredentialsPasswordKey: credentials[constants.DatabaseCredentialsPasswordKey],
// 		},
// 	}

// 	return defaultSecret

// }

func checkEmptyOrNull(valueToCheck interface{}) bool {

	if utils.IsZeroOfUnderlyingType(valueToCheck) {
		return true
	}

	return false
}

/*
func setValueInt32(valueToCheck *int32, defaultValue *int32) *int32 {

	if utils.IsZeroOfUnderlyingType(valueToCheck) {
		return defaultValue
	}

	return valueToCheck
}
*/
