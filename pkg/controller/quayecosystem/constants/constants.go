package constants

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// OperatorName is a operator name
	OperatorName = "quay-operator"
	// QuayImage is the Quay image
	QuayImage = "quay.io/redhat/quay:v3.0.2"
	// ImagePullSecret is the name of the image pull secret for retrieving images from a protected image registry
	ImagePullSecret = "redhat-pull-secret"
	// RedisImage is the name of the Redis Image
	RedisImage = "quay.io/quay/redis:latest"
	// LabelAppKey is the name of the label key
	LabelAppKey = "app"
	// LabelAppValue is the name of the label
	LabelAppValue = OperatorName
	// LabelCompoentKey com
	LabelCompoentKey = "quay-enterprise-component"
	// LabelComponentAppValue is the name of the app label
	LabelComponentAppValue = "app"
	// LabelComponentConfigValue is the name of the config label
	LabelComponentConfigValue = "config"
	// LabelComponentRedisValue is the name of the Redis label
	LabelComponentRedisValue = "redis"
	// LabelComponentQuayDatabaseValue is the name of the Quay database label
	LabelComponentQuayDatabaseValue = "quay-database"
	// LabelQuayCRKey is the label name of the quay custom resource
	LabelQuayCRKey = "quay-enterprise-cr"
	// AnyUIDSCC is the name of the anyuid SCC
	AnyUIDSCC = "anyuid"
	// RedisServiceAccount is the name of the Redis ServiceAccount
	RedisServiceAccount = "redis"
	// QuayServiceAccount is the name of the Quay ServiceAccount
	QuayServiceAccount = "quay"
	// PostgresqlName is the name used to represent PostgreSQL
	PostgresqlName = "postgresql"
	// PostgresqlImage is the Postgresql image
	PostgresqlImage = "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1"
	// PostgreSQLPort is the database port for PostgreSQL
	PostgreSQLPort = 5432
	// QuayDatabaseMemory is the default memory amount
	QuayDatabaseMemory = "512Mi"
	// QuayDatabaseCPU is the default CPU amount
	QuayDatabaseCPU = "300m"
	// QuayDatabaseName is the default database name
	QuayDatabaseName = "quay"
	// QuayDatabasePVCSize is the size of the PVC for Quay
	QuayDatabasePVCSize = "1Gi"

	// DatabaseCredentialsUsernameKey represents the key for the database username
	DatabaseCredentialsUsernameKey = "database-username"
	// DatabaseCredentialsPasswordKey represents the key for the database password
	DatabaseCredentialsPasswordKey = "database-password"
	// DatabaseCredentialsDatabaseKey represents the key for the database name
	DatabaseCredentialsDatabaseKey = "database-name"
	// DatabaseCredentialsRootPasswordKey represents the key for the database root password
	DatabaseCredentialsRootPasswordKey = "database-root-password"
	// QuayDatabaseCredentialsDefaultUsername represents the default database username
	QuayDatabaseCredentialsDefaultUsername = "quay"
	// QuayDatabaseCredentialsDefaultPassword represents the default database password
	QuayDatabaseCredentialsDefaultPassword = "quay"
	// QuayDatabaseCredentialsDefaultRootPassword represents the default database password
	QuayDatabaseCredentialsDefaultRootPassword = "quayAdmin"
	// QuayDatabaseCredentialsDefaultDatabaseName represents the default database name
	QuayDatabaseCredentialsDefaultDatabaseName = "quay"

	// ClairDatabaseCredentialsDefaultUsername represents the default database username
	ClairDatabaseCredentialsDefaultUsername = "clair"
	// ClairDatabaseCredentialsDefaultPassword represents the default database password
	ClairDatabaseCredentialsDefaultPassword = "clair"
	// ClairDatabaseCredentialsDefaultRootPassword represents the default database password
	ClairDatabaseCredentialsDefaultRootPassword = "clairAdmin"
	// ClairDatabaseCredentialsDefaultDatabaseName represents the default database name
	ClairDatabaseCredentialsDefaultDatabaseName = "clair"

	// QuayRegistryStorageDirectory represents the location where registry storage is mounted in the container
	QuayRegistryStorageDirectory = "/datastorage/registry"
	// QuayRegistryStoragePersistentVolumeStoreSize represents the size of the PersistentVolume that should be used for registry storage
	QuayRegistryStoragePersistentVolumeStoreSize = "20Gi"
	// QuayEntryName represents the name of the operation to execute
	QuayEntryName = "QUAYENTRY"
	// QuayEntryConfigValue represents the value that will start the Quay config container
	QuayEntryConfigValue = "config"
	// QuayConfigUsername represents the username of the Quay config container
	QuayConfigUsername = "quayconfig"
	// QuayConfigPasswordName represents the name of the environment variable contining the Quay configuration password
	QuayConfigPasswordName = "CONFIG_APP_PASSWORD"
	// QuayConfigPasswordKey represents the key for the Quay Config secret
	QuayConfigPasswordKey = "config-app-password"
	// QuayConfigSecretName represents the name of the Quay Config secret
	QuayConfigSecretName = "quay-config"
	// QuayConfigDefaultPasswordValue is the default password for the Quay Config endpoint
	QuayConfigDefaultPasswordValue = "quay"
	// QuayContainerConfigName represents the name of the Quay config container
	QuayContainerConfigName = "quay-enterprise-config"
	// QuayContainerAppName represents the name of the Quay app container
	QuayContainerAppName = "quay-enterprise-app"
	// QuaySuperuserUsernameKey represents the key for the superuser username
	QuaySuperuserUsernameKey = "superuser-username"
	// QuaySuperuserPasswordKey represents the key for the superuser password
	QuaySuperuserPasswordKey = "superuser-password"
	// QuaySuperuserEmailKey represents the key for the superuser email
	QuaySuperuserEmailKey = "superuser-email"
	// QuaySuperuserSecretName represents the name of the secret containing the quay superuser details
	QuaySuperuserSecretName = "quay-superuser"
	// QuaySuperuserDefaultUsername represents the default Quay superuser username
	QuaySuperuserDefaultUsername = "quay"
	// QuaySuperuserDefaultPassword represents the default Quay superuser password
	QuaySuperuserDefaultPassword = "password"
	// QuaySuperuserDefaultEmail represents the default Quay superuser password
	QuaySuperuserDefaultEmail = "quay@redhat.com"
)

var (
	OneInt int32 = 1

	// DefaultQuaySuperuserCredentials represents a map containing the default values for the Quay Superuser
	DefaultQuaySuperuserCredentials = map[string]string{
		DatabaseCredentialsPasswordKey:     QuaySuperuserDefaultUsername,
		DatabaseCredentialsDatabaseKey:     QuaySuperuserDefaultPassword,
		DatabaseCredentialsRootPasswordKey: QuaySuperuserDefaultEmail,
	}
	// DefaultQuayDatabaseCredentials represents a map containing the default values for Quay database
	DefaultQuayDatabaseCredentials = map[string]string{
		DatabaseCredentialsUsernameKey:     QuayDatabaseCredentialsDefaultUsername,
		DatabaseCredentialsPasswordKey:     QuayDatabaseCredentialsDefaultPassword,
		DatabaseCredentialsDatabaseKey:     QuayDatabaseCredentialsDefaultDatabaseName,
		DatabaseCredentialsRootPasswordKey: QuayDatabaseCredentialsDefaultRootPassword,
	}
	// DefaultClairDatabaseCredentials represents a map containing the default values for Clair database
	DefaultClairDatabaseCredentials = map[string]string{
		DatabaseCredentialsUsernameKey:     ClairDatabaseCredentialsDefaultUsername,
		DatabaseCredentialsPasswordKey:     ClairDatabaseCredentialsDefaultUsername,
		DatabaseCredentialsDatabaseKey:     ClairDatabaseCredentialsDefaultDatabaseName,
		DatabaseCredentialsRootPasswordKey: ClairDatabaseCredentialsDefaultRootPassword,
	}

	// RequiredDatabaseCredentialKeys represents the keys that are required for a provided database credential
	RequiredDatabaseCredentialKeys = []string{DatabaseCredentialsUsernameKey, DatabaseCredentialsPasswordKey, DatabaseCredentialsDatabaseKey}

	// DefaultQuayConfigCredentials represents a map containing the default Quay Config
	DefaultQuayConfigCredentials = map[string]string{
		QuayConfigPasswordKey: QuayConfigDefaultPasswordValue,
	}

	// RedisReplicas is the port number for Redis
	RedisReplicas int32 = 1
	// RedisPort is the port number for Redis
	RedisPort int32 = 6379

	// QuayRegistryStoragePersistentVolumeAccessModes represents the access modes for the registry storage persistent volume
	QuayRegistryStoragePersistentVolumeAccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
)
