package kustomize

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/yaml"

	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/distributedstorage"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/hostsettings"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/repomirror"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/securityscanner"
	"github.com/quay/config-tool/pkg/lib/shared"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

func quayRegistry(name string) *v1.QuayRegistry {
	return &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
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

var fieldGroupForTests = []struct {
	name      string
	component v1.ComponentKind
	quay      *v1.QuayRegistry
	ctx       quaycontext.QuayRegistryContext
	expected  shared.FieldGroup
}{
	{
		"clair",
		"clair",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{},
		&securityscanner.SecurityScannerFieldGroup{
			FeatureSecurityScanner:              true,
			SecurityScannerIndexingInterval:     30,
			SecurityScannerNotifications:        true,
			SecurityScannerV4Endpoint:           "http://test-clair-app:80",
			SecurityScannerV4NamespaceWhitelist: []string{"admin"},
			SecurityScannerV4PSK:                "abc123",
		},
	},
	{
		"redis",
		"redis",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{},
		&redis.RedisFieldGroup{
			BuildlogsRedis: &redis.BuildlogsRedisStruct{
				Host: "test-quay-redis",
				Port: 6379,
			},
			UserEventsRedis: &redis.UserEventsRedisStruct{
				Host: "test-quay-redis",
				Port: 6379,
			},
		},
	},
	{
		"postgres",
		"postgres",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{
			DbUri: "postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database",
		},
		&database.DatabaseFieldGroup{
			DbUri: "postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database",
			DbConnectionArgs: &database.DbConnectionArgsStruct{
				Autorollback: true,
				Threadlocals: true,
			},
		},
	},
	{
		"objectstorage",
		"objectstorage",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			StorageBucketName:     "quay-datastore",
			StorageHostname:       "s3.noobaa.svc",
			StorageAccessKey:      "abc123",
			StorageSecretKey:      "super-secret",
		},
		&distributedstorage.DistributedStorageFieldGroup{
			FeatureProxyStorage:                true,
			DistributedStoragePreference:       []string{"local_us"},
			DistributedStorageDefaultLocations: []string{"local_us"},
			DistributedStorageConfig: map[string]*distributedstorage.DistributedStorageDefinition{
				"local_us": {
					Name: "RHOCSStorage",
					Args: &shared.DistributedStorageArgs{
						AccessKey:   "abc123",
						BucketName:  "quay-datastore",
						Hostname:    "s3.noobaa.svc",
						IsSecure:    true,
						Port:        443,
						SecretKey:   "super-secret",
						StoragePath: "/datastorage/registry",
					},
				},
			},
		},
	},
	{
		"route",
		"route",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
			ServerHostname:  "test-quay-ns.apps.example.com",
		},
		&hostsettings.HostSettingsFieldGroup{
			ServerHostname:         "test-quay-ns.apps.example.com",
			ExternalTlsTermination: true,
			PreferredUrlScheme:     "https",
		},
	},
	{
		"horizontalpodautoscaler",
		"horizontalpodautoscaler",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{},
		nil,
	},
	{
		"mirror",
		"mirror",
		quayRegistry("test"),
		quaycontext.QuayRegistryContext{},
		&repomirror.RepoMirrorFieldGroup{
			FeatureRepoMirror:   true,
			RepoMirrorInterval:  30,
			RepoMirrorTlsVerify: true,
		},
	},
}

func TestFieldGroupFor(t *testing.T) {
	assert := assert.New(t)

	for _, test := range fieldGroupForTests {
		fieldGroup, err := FieldGroupFor(&test.ctx, test.component, test.quay)
		assert.Nil(err, test.name)

		// TODO: Make this more generic for other randomly-generated fields.
		if test.name == "clair" {
			secscanFieldGroup := fieldGroup.(*securityscanner.SecurityScannerFieldGroup)

			assert.True(len(secscanFieldGroup.SecurityScannerV4PSK) > 0, test.name)
			secscanFieldGroup.SecurityScannerV4PSK = "abc123"
		}

		expected, err := yaml.Marshal(test.expected)
		assert.Nil(err, test.name)
		configFields, err := yaml.Marshal(fieldGroup)
		assert.Nil(err, test.name)

		assert.Equal(string(expected), string(configFields), test.name)
	}
}

var containsComponentConfigTests = []struct {
	name          string
	component     v1.ComponentKind
	managed       bool
	rawConfig     string
	expected      bool
	expectedError error
}{
	{
		"ClairContains",
		"clair",
		true,
		`FEATURE_SECURITY_SCANNER: true`,
		true,
		nil,
	},
	{
		"ClairDoesNotContain",
		"clair",
		true,
		``,
		false,
		nil,
	},
	{
		"PostgresContains",
		"postgres",
		true,
		`DB_URI: postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database`,
		true,
		nil,
	},
	{
		"PostgresDoesNotContain",
		"postgres",
		true,
		``,
		false,
		nil,
	},
	{
		"RedisContains",
		"redis",
		true,
		`BUILDLOGS_REDIS:
  host: test-quay-redis
`,
		true,
		nil,
	},
	{
		"RedisDoesNotContain",
		"redis",
		true,
		``,
		false,
		nil,
	},
	{
		"ObjectStorageContains",
		"objectstorage",
		true,
		`DISTRIBUTED_STORAGE_PREFERENCE: 
  - local_us
`,
		true,
		nil,
	},
	{
		"ObjectStorageDoesNotContain",
		"objectstorage",
		true,
		``,
		false,
		nil,
	},
	{
		"MirrorContains",
		"mirror",
		true,
		`FEATURE_REPO_MIRROR: true`,
		true,
		nil,
	},
	{
		"MirrorDeosNotContain",
		"mirror",
		true,
		``,
		false,
		nil,
	},
	{
		"RouteContains",
		"route",
		true,
		`PREFERRED_URL_SCHEME: http`,
		true,
		nil,
	},
	{
		"RouteContainsServerHostname",
		"route",
		true,
		`SERVER_HOSTNAME: registry.skynet.com`,
		true,
		nil,
	},
	{
		"RouteDoesNotContain",
		"route",
		true,
		``,
		false,
		nil,
	},
	{
		"RouteUnmanagedDoesNotContain",
		"route",
		false,
		``,
		false,
		nil,
	},
	{
		"RouteUnmanagedContainsServerHostname",
		"route",
		false,
		`SERVER_HOSTNAME: registry.skynet.com`,
		true,
		nil,
	},
	{
		"HorizontalPodAutoscalerDoesNotContain",
		"horizontalpodautoscaler",
		true,
		``,
		false,
		nil,
	},
}

func TestContainsComponentConfig(t *testing.T) {
	assert := assert.New(t)

	for _, test := range containsComponentConfigTests {
		var fullConfig map[string]interface{}
		var certs map[string][]byte
		err := yaml.Unmarshal([]byte(test.rawConfig), &fullConfig)
		assert.Nil(err, test.name)

		contains, err := ContainsComponentConfig(fullConfig, certs, v1.Component{Kind: test.component, Managed: test.managed})

		if test.expectedError != nil {
			assert.NotNil(err, test.name)
		} else {
			assert.Nil(err, test.name)
			assert.Equal(test.expected, contains, test.name)
		}
	}
}

func certKeyPairFor(hostname string, alternateHostnames []string) [][]byte {
	cert, key, err := cert.GenerateSelfSignedCertKey(hostname, nil, alternateHostnames)
	if err != nil {
		panic(err)
	}

	return [][]byte{cert, key}
}

var ensureTLSForTests = []struct {
	name                 string
	routeManaged         bool
	serverHostname       string
	buildManagerHostname string
	providedCertKeyPair  [][]byte
	expectedErr          error
}{
	{
		"ManagedRouteNoHostnameNoCerts",
		true,
		"",
		"",
		[][]byte{nil, nil},
		nil,
	},
	{
		"ManagedRouteProvidedHostnameProvidedIncorrectCerts",
		true,
		"registry.company.com",
		"",
		certKeyPairFor("nonexistent.company.com", nil),
		fmt.Errorf("provided certificate/key pair not valid for host 'registry.company.com': x509: certificate is valid for nonexistent.company.com, not registry.company.com"),
	},
	{
		"ManagedRouteProvidedHostnameNoCerts",
		true,
		"registry.company.com",
		"",
		[][]byte{nil, nil},
		nil,
	},
	{
		"ManagedRouteProvidedHostnameProvidedCerts",
		true,
		"registry.company.com",
		"",
		certKeyPairFor("registry.company.com", nil),
		nil,
	},
	{
		"ManagedRouteProvidedBuildmanagerHostnameProvidedIncorrectCerts",
		true,
		"registry.company.com",
		"builds.company.com",
		certKeyPairFor("registry.company.com", nil),
		fmt.Errorf("provided certificate/key pair not valid for host 'builds.company.com': x509: certificate is valid for registry.company.com, not builds.company.com"),
	},
	{
		"ManagedRouteProvidedBuildmanagerHostnameProvidedCerts",
		true,
		"registry.company.com",
		"builds.company.com",
		certKeyPairFor("registry.company.com", []string{"builds.company.com"}),
		nil,
	},
	{
		"ManagedRouteProvidedBuildmanagerHostnameNoCerts",
		true,
		"registry.company.com",
		"builds.company.com",
		[][]byte{nil, nil},
		nil,
	},
}

func TestEnsureTLSFor(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureTLSForTests {
		quayRegistry := quayRegistry("test")

		quayContext := quaycontext.QuayRegistryContext{
			ServerHostname:       test.serverHostname,
			BuildManagerHostname: test.buildManagerHostname,
			TLSCert:              test.providedCertKeyPair[0],
			TLSKey:               test.providedCertKeyPair[1],
		}

		tlsCert, tlsKey, err := EnsureTLSFor(&quayContext, quayRegistry)

		assert.Equal(test.expectedErr, err, test.name)

		if test.expectedErr == nil {
			if test.providedCertKeyPair[0] != nil && test.providedCertKeyPair[1] != nil {
				assert.Equal(string(test.providedCertKeyPair[0]), string(tlsCert), test.name)
				assert.Equal(string(test.providedCertKeyPair[1]), string(tlsKey), test.name)
			}

			shared.ValidateCertPairWithHostname(tlsCert, tlsKey, test.serverHostname, fieldGroupNameFor("route"))

			if test.buildManagerHostname != "" {
				shared.ValidateCertPairWithHostname(tlsCert, tlsKey, test.buildManagerHostname, fieldGroupNameFor("route"))
			}
		}
	}
}
