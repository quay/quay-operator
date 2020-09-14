package kustomize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/quay/quay-operator/api/v1"
)

func quayRegistry(name string) *v1.QuayRegistry {
	return &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1.SupportsObjectStorageAnnotation: "true",
				v1.StorageHostnameAnnotation:       "s3.noobaa.svc",
				v1.StorageBucketNameAnnotation:     "quay-datastore",
				v1.StorageAccessKeyAnnotation:      "abc123",
				v1.StorageSecretKeyAnnotation:      "super-secret",
			},
		},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true},
				{Kind: "clair", Managed: true},
				{Kind: "redis", Managed: true},
				{Kind: "objectstorage", Managed: true},
			},
		},
	}
}

var secretForTests = []struct {
	name      string
	component string
	quay      *v1.QuayRegistry
	expected  []byte
}{
	{
		"clair",
		"clair",
		quayRegistry("test"),
		[]byte(`FEATURE_SECURITY_SCANNER: true
SECURITY_SCANNER_ENDPOINT: ""
SECURITY_SCANNER_INDEXING_INTERVAL: 30
SECURITY_SCANNER_NOTIFICATIONS: false
SECURITY_SCANNER_V4_ENDPOINT: http://test-clair
SECURITY_SCANNER_V4_NAMESPACE_WHITELIST:
- admin
`),
	},
	{
		"redis",
		"redis",
		quayRegistry("test"),
		[]byte(`BUILDLOGS_REDIS:
  host: test-quay-redis
  password: ""
  port: 6379
USER_EVENTS_REDIS:
  host: test-quay-redis
  password: ""
  port: 6379
`),
	},
	{
		"postgres",
		"postgres",
		quayRegistry("test"),
		[]byte(`DB_CONNECTION_ARGS:
  autorollback: true
  threadlocals: true
DB_URI: postgresql://postgres:postgres@test-quay-postgres/quay
`),
	},
	{
		"objectstorage",
		"objectstorage",
		quayRegistry("test"),
		[]byte(`DISTRIBUTED_STORAGE_CONFIG:
  local_us:
  - RadosGWStorage
  - access_key: abc123
    bucket_name: quay-datastore
    hostname: s3.noobaa.svc
    is_secure: true
    port: 443
    secret_key: super-secret
    storage_path: /datastorage/registry
DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
- local_us
DISTRIBUTED_STORAGE_PREFERENCE:
- local_us
FEATURE_PROXY_STORAGE: true
FEATURE_STORAGE_REPLICATION: false
`),
	},
}

func TestConfigFileFor(t *testing.T) {
	assert := assert.New(t)

	for _, test := range secretForTests {
		result, err := ConfigFileFor(test.component, test.quay)

		assert.Nil(err)
		assert.Equal(string(test.expected), string(result))
	}
}
