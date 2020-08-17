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
		},
		Spec: v1.QuayRegistrySpec{
			ManagedComponents: []v1.ManagedComponent{
				{Kind: "postgres"},
				{Kind: "clair"},
				{Kind: "redis"},
				{Kind: "storage"},
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
		[]byte(`DB_URI: postgresql://postgres:postgres@test-quay-postgres/quay
`),
	},
	{
		"storage",
		"storage",
		quayRegistry("test"),
		[]byte(`DISTRIBUTED_STORAGE_CONFIG:
  default:
  - RadosGWStorage
  - access_key: minio
    bucket_name: quay-datastore
    hostname: test-quay-datastore
    is_secure: false
    port: 9000
    secret_key: minio123
    storage_path: /datastorage/registry
DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
- default
DISTRIBUTED_STORAGE_PREFERENCE:
- default
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
