package validation

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetDefaults(client client.Client, quayConfiguration *resources.QuayConfiguration) bool {

	changed := false

	// Initialize Base variables and objects
	quayConfiguration.QuayConfigUsername = constants.QuayConfigUsername
	quayConfiguration.QuayConfigPassword = constants.QuayConfigDefaultPasswordValue
	quayConfiguration.QuaySuperuserUsername = constants.QuaySuperuserDefaultUsername
	quayConfiguration.QuaySuperuserPassword = constants.QuaySuperuserDefaultPassword
	quayConfiguration.QuaySuperuserEmail = constants.QuaySuperuserDefaultEmail
	quayConfiguration.QuayConfigPasswordSecret = resources.GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)
	quayConfiguration.QuayDatabase.Username = constants.QuayDatabaseCredentialsDefaultUsername
	quayConfiguration.QuayDatabase.Password = constants.QuayDatabaseCredentialsDefaultPassword
	quayConfiguration.QuayDatabase.Database = constants.QuayDatabaseCredentialsDefaultDatabaseName
	quayConfiguration.QuayDatabase.RootPassword = constants.QuayDatabaseCredentialsDefaultRootPassword
	quayConfiguration.QuayDatabase.Server = resources.GetDatabaseResourceName(quayConfiguration.QuayEcosystem, constants.DatabaseComponentQuay)
	quayConfiguration.ClairDatabase.Username = constants.ClairDatabaseCredentialsDefaultUsername
	quayConfiguration.ClairDatabase.Password = constants.ClairDatabaseCredentialsDefaultPassword
	quayConfiguration.ClairDatabase.Server = resources.GetDatabaseResourceName(quayConfiguration.QuayEcosystem, constants.DatabaseComponentClair)
	quayConfiguration.ClairDatabase.Database = constants.ClairDatabaseCredentialsDefaultDatabaseName
	quayConfiguration.ClairDatabase.RootPassword = constants.ClairDatabaseCredentialsDefaultRootPassword
	quayConfiguration.ClairUpdateInterval = constants.ClairDefaultUpdateInterval

	if quayConfiguration.QuayEcosystem.Spec.Quay == nil {
		quayConfiguration.QuayEcosystem.Spec.Quay = &redhatcopv1alpha1.Quay{}
		changed = true
	}

	if quayConfiguration.QuayEcosystem.Spec.Quay.Database == nil {
		quayConfiguration.QuayEcosystem.Spec.Quay.Database = &redhatcopv1alpha1.Database{}
		changed = true
	}

	if quayConfiguration.QuayEcosystem.Spec.Redis == nil {
		quayConfiguration.QuayEcosystem.Spec.Redis = &redhatcopv1alpha1.Redis{}
		changed = true
	}

	// Core Quay Values
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Image) {
		changed = true
		quayConfiguration.QuayEcosystem.Spec.Quay.Image = constants.QuayImage
	}

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.DeploymentStrategy) {
		changed = true
		quayConfiguration.QuayEcosystem.Spec.Quay.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
	}

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ReadinessProbe) {
		changed = true
		quayConfiguration.QuayEcosystem.Spec.Quay.ReadinessProbe = getDefaultQuayReadinessProbe()
	}

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.LivenessProbe) {
		changed = true
		quayConfiguration.QuayEcosystem.Spec.Quay.LivenessProbe = getDefaultQuayLivenessProbe()
	}

	// Default Quay Config Route
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ConfigRouteHost) {
		quayConfiguration.QuayConfigHostname = resources.GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)
	} else {
		quayConfiguration.QuayConfigHostname = quayConfiguration.QuayEcosystem.Spec.Quay.ConfigRouteHost
	}

	// Apply default values for Redis
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Image) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Redis.Image = constants.RedisImage
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.DeploymentStrategy) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Redis.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.ReadinessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Redis.ReadinessProbe = getDefaultRedisReadinessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.LivenessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Redis.LivenessProbe = getDefaultRedisLivenessProbe()
		}

	}

	// Set Redis Hostname
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		quayConfiguration.RedisHostname = resources.GetRedisResourcesName(quayConfiguration.QuayEcosystem)
	} else {
		quayConfiguration.RedisHostname = quayConfiguration.QuayEcosystem.Spec.Redis.Hostname
	}

	// Set Redis Port
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Port) {
		quayConfiguration.RedisPort = &constants.RedisPort
	} else {
		quayConfiguration.RedisPort = quayConfiguration.QuayEcosystem.Spec.Redis.Port
	}

	// User would like to have a database automatically provisioned if server not provided
	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server) {

		quayConfiguration.QuayDatabase.Server = resources.GetDatabaseResourceName(quayConfiguration.QuayEcosystem, constants.DatabaseComponentQuay)

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image = constants.PostgresqlImage
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.DeploymentStrategy) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Quay.Database.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.ReadinessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Quay.Database.ReadinessProbe = getDefaultDatabaseReadinessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.LivenessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Quay.Database.LivenessProbe = getDefaultDatabaseLivenessProbe()
		}

	} else {
		quayConfiguration.QuayDatabase.Server = quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server

	}

	// Clair Core Values
	if quayConfiguration.QuayEcosystem.Spec.Clair != nil && quayConfiguration.QuayEcosystem.Spec.Clair.Enabled {

		// Add Clair Service Account to List of SCC's
		constants.RequiredAnyUIDSccServiceAccounts = append(constants.RequiredAnyUIDSccServiceAccounts, constants.ClairServiceAccount)

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Image) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.Image = constants.ClairImage
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.ReadinessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.ReadinessProbe = getDefaultClairReadinessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.LivenessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.LivenessProbe = getDefaultClairLivenessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.DeploymentStrategy) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
		}

		if quayConfiguration.QuayEcosystem.Spec.Clair.Database == nil {
			quayConfiguration.QuayEcosystem.Spec.Clair.Database = &redhatcopv1alpha1.Database{}
			changed = true
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Database.ReadinessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.Database.ReadinessProbe = getDefaultDatabaseReadinessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Database.LivenessProbe) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.Database.LivenessProbe = getDefaultDatabaseLivenessProbe()
		}

		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.DeploymentStrategy) {
			changed = true
			quayConfiguration.QuayEcosystem.Spec.Clair.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
		}

		// User would like to have a database automatically provisioned if server not provided
		if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Database.Server) {

			quayConfiguration.ClairDatabase.Server = resources.GetDatabaseResourceName(quayConfiguration.QuayEcosystem, constants.DatabaseComponentClair)

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Database.Image) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Clair.Database.Image = constants.PostgresqlImage
			}

			if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.Database.DeploymentStrategy) {
				changed = true
				quayConfiguration.QuayEcosystem.Spec.Clair.Database.DeploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
			}

		} else {
			quayConfiguration.ClairDatabase.Server = quayConfiguration.QuayEcosystem.Spec.Clair.Database.Server
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

	if updatedRegistryBakends, registryBakendsChanged := setDefaultBackendSourceValues(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends); registryBakendsChanged {
		quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends = updatedRegistryBakends
		changed = true
	}

	return changed
}

func getDefaultClairReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		TimeoutSeconds:      5,
		FailureThreshold:    3,
		InitialDelaySeconds: 10,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   constants.ClairHealthEndpoint,
				Port:   intstr.IntOrString{IntVal: 6061},
				Scheme: "HTTP",
			},
		},
	}
}

func getDefaultClairLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		TimeoutSeconds:      5,
		FailureThreshold:    3,
		InitialDelaySeconds: 30,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   constants.ClairHealthEndpoint,
				Port:   intstr.IntOrString{IntVal: 6061},
				Scheme: "HTTP",
			},
		},
	}
}

func getDefaultQuayReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 5,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   constants.QuayHealthEndpoint,
				Port:   intstr.IntOrString{IntVal: 8443},
				Scheme: "HTTPS",
			},
		},
	}
}

func getDefaultQuayLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		TimeoutSeconds:      5,
		FailureThreshold:    3,
		InitialDelaySeconds: 120,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   constants.QuayHealthEndpoint,
				Port:   intstr.IntOrString{IntVal: 8443},
				Scheme: "HTTPS",
			},
		},
	}
}

func getDefaultDatabaseReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"/usr/libexec/check-container"},
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 10,
		TimeoutSeconds:      1,
	}
}

func getDefaultDatabaseLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"/usr/libexec/check-container", "--live"},
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 120,
		TimeoutSeconds:      10,
	}
}

func getDefaultRedisReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 30,
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{IntVal: 6379},
			},
		},
	}
}

func getDefaultRedisLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 30,
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{IntVal: 6379},
			},
		},
	}
}

func setDefaultBackendSourceValues(registryBackends []redhatcopv1alpha1.RegistryBackend) ([]redhatcopv1alpha1.RegistryBackend, bool) {

	changed := false

	for _, registryBackend := range registryBackends {

		if !utils.IsZeroOfUnderlyingType(registryBackend.Local) {
			if utils.IsZeroOfUnderlyingType(registryBackend.Local.StoragePath) {
				changed = true
				registryBackend.Local.StoragePath = constants.QuayRegistryStoragePath
			}
			continue
		}

		if !utils.IsZeroOfUnderlyingType(registryBackend.S3) {
			if utils.IsZeroOfUnderlyingType(registryBackend.S3.StoragePath) {
				changed = true
				registryBackend.S3.StoragePath = constants.QuayRegistryStoragePath
			}
			continue
		}
	}

	return registryBackends, changed

}
