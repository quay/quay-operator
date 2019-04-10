package constants

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// OperatorName is a operator name
	OperatorName = "quay-operator"
	// QuayImage is the Quay image
	QuayImage = "quay.io/coreos/quay:v2.9.3"
	// ImagePullSecret is the name of the image pull secret for retrieving images from a protected image registry
	ImagePullSecret = "coreos-pull-secret"
	// RedisImage is the name of the Redis Image
	RedisImage = "quay.io/quay/redis:latest"
	// LabelAppKey is the name of the label key
	LabelAppKey = "app"
	// LabelAppValue is the name of the label
	LabelAppValue = OperatorName
	// LabelCompoentKey com
	LabelCompoentKey = OperatorName + "-component"
	// LabelComponentAppValue is the name of the app label
	LabelComponentAppValue = "app"
	// LabelComponentRedisValue is the name of the Redis label
	LabelComponentRedisValue = "redis"
	// LabelComponentQuayDatabaseValue is the name of the Quay database label
	LabelComponentQuayDatabaseValue = "quay-database"
	// LabelQuayCRKey is the label name of the quay custom resource
	LabelQuayCRKey = "quay-enterprise-cr"
	// AnyUIDSCC is the name of the anyuid SCC
	AnyUIDSCC = "anyuid"
	// QuayEcosystemServiceAccount is the name of the Quay ServiceAccount
	QuayEcosystemServiceAccount = "quayecosystem"
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

	// QuayRegistryStoragePersistentVolumeStoreSize represents the size of the PersistentVolume that should be used for registry storage
	QuayRegistryStoragePersistentVolumeStoreSize, _ = resource.ParseQuantity("20Gi")
)
