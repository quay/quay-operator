package constants

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
	// LabelQuayCRKey is the label name of the quay custom resource
	LabelQuayCRKey = "quay-enterprise-cr"
	// AnyUIDSCC is the name of the anyuid SCC
	AnyUIDSCC = "anyuid"
	// QuayEcosystemServiceAccount is the name of the Quay ServiceAccount
	QuayEcosystemServiceAccount = "quayecosystem"
)
