package constants

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// OperatorName is a operator name
	OperatorName = "quay-operator"
	// QuayImage is the Quay image
	QuayImage = "quay.io/redhat/quay:v3.0.1"
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
	// MySQLName is the name used to represent MySQL
	MySQLName = "mysql"
	// PostgresqlName is the name used to represent MySQL
	PostgresqlName = "postgresql"
	// PostgresqlImage is the Postgresql image
	PostgresqlImage = "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1"
	// PostgreSQLPort is the database port for MySQL
	PostgreSQLPort = 5432
	// MySQLImage is the PostgreSQL image
	MySQLImage = "registry.access.redhat.com/rhscl/mysql-57-rhel7:5.7"
	// MySQLPort is the database port for MySQL
	MySQLPort = 3306
	// DatabaseMemory is the default memory amount
	DatabaseMemory = "512Mi"
	// DatabaseCPU is the default CPU amount
	DatabaseCPU = "300m"
	// QuayDatabaseName is the default database name
	QuayDatabaseName = "quay"
	// QuayPVCSize is the size of the PVC for Quay
	QuayPVCSize = "1Gi"
	// DatabaseCredentialsUsernameKey represents the key for the database username
	DatabaseCredentialsUsernameKey = "database-username"
	// DatabaseCredentialsPasswordKey represents the key for the database password
	DatabaseCredentialsPasswordKey = "database-password"
	// DatabaseCredentialsDatabaseKey represents the key for the database name
	DatabaseCredentialsDatabaseKey = "database-name"
	// DatabaseCredentialsRootPasswordKey represents the key for the database root password
	DatabaseCredentialsRootPasswordKey = "database-root-password"
	// QuayRegistryStorageDirectory represents the location where registry storage is mounted in the container
	QuayRegistryStorageDirectory = "/datastorage/registry"
	// QuayRegistryStoragePersistentVolumeStoreSize represents the size of the PersistentVolume that should be used for registry storage
	QuayRegistryStoragePersistentVolumeStoreSize = "20Gi"
	// QuayEntryName represents the name of the operation to execute
	QuayEntryName = "QUAYENTRY"
	// QuayEntryConfigValue represents the value that will start the Quay config container
	QuayEntryConfigValue = "config"
	// QuayConfigPasswordName represents the name of the environment variable contining the Quay configuration password
	QuayConfigPasswordName = "CONFIG_APP_PASSWORD"
	// QuayContainerConfigName represents the name of the Quay config container
	QuayContainerConfigName = "quay-enterprise-config"
	// QuayContainerAppName represents the name of the Quay app container
	QuayContainerAppName = "quay-enterprise-app"
)

var (
	// DefaultQuayDatabaseCredentials represents a map containing the default values for Quay database
	DefaultQuayDatabaseCredentials = map[string]string{
		DatabaseCredentialsUsernameKey:     "quay",
		DatabaseCredentialsPasswordKey:     "quayPassword",
		DatabaseCredentialsDatabaseKey:     "quay",
		DatabaseCredentialsRootPasswordKey: "quayAdmin",
	}
	// DefaultClairDatabaseCredentials represents a map containing the default values for Clair database
	DefaultClairDatabaseCredentials = map[string]string{
		DatabaseCredentialsUsernameKey:     "clair",
		DatabaseCredentialsPasswordKey:     "clairPassword",
		DatabaseCredentialsDatabaseKey:     "clair",
		DatabaseCredentialsRootPasswordKey: "clairAdmin",
	}

	// QuayRegistryStoragePersistentVolumeAccessModes represents the access modes for the registry storage persistent volume
	QuayRegistryStoragePersistentVolumeAccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
)
