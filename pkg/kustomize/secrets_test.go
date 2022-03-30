package kustomize

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/yaml"

	"github.com/quay/clair/config"
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
			SecurityScannerV4Endpoint:           "http://test-clair-app.ns.svc.cluster.local",
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
	cfgbundle     map[string][]byte
	expected      bool
	expectedError error
}{
	{
		name:      "ClairContains",
		component: "clair",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`FEATURE_SECURITY_SCANNER: true`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "ClairDoesNotContain",
		component: "clair",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected: false,
	},
	{
		name:      "PostgresContains",
		component: "postgres",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`DB_URI: postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "PostgresDoesNotContain",
		component: "postgres",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "RedisContains",
		component: "redis",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte("BUILDLOGS_REDIS:\n  host: test-quay-redis"),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "RedisDoesNotContain",
		component: "redis",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "ObjectStorageContains",
		component: "objectstorage",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte("DISTRIBUTED_STORAGE_PREFERENCE:\n- local_us"),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "ObjectStorageDoesNotContain",
		component: "objectstorage",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "MirrorContains",
		component: "mirror",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`FEATURE_REPO_MIRROR: true`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "MirrorDeosNotContain",
		component: "mirror",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "RouteContains",
		component: "route",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`PREFERRED_URL_SCHEME: http`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "RouteContainsServerHostname",
		component: "route",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`SERVER_HOSTNAME: registry.skynet.com`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "RouteDoesNotContain",
		component: "route",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "RouteUnmanagedDoesNotContain",
		component: "route",
		managed:   false,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "RouteUnmanagedContainsServerHostname",
		component: "route",
		managed:   false,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(`SERVER_HOSTNAME: registry.skynet.com`),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "HorizontalPodAutoscalerDoesNotContain",
		component: "horizontalpodautoscaler",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "ClairDatabaseComponentDoesNotContainFields",
		component: "clairpostgres",
		managed:   true,
		cfgbundle: map[string][]byte{
			"clair-config.yaml": []byte(`http_listen_addr: ":8090"`),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "ClairDatabaseComponentDoesNotContainClairConfig",
		component: "clairpostgres",
		managed:   true,
		cfgbundle: map[string][]byte{
			"config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
	{
		name:      "ContainsClairPostgresConfiguration",
		component: "clairpostgres",
		managed:   false,
		cfgbundle: map[string][]byte{
			"clair-config.yaml": []byte("indexer:\n connstring: 'some fake dsn'"),
		},
		expected:      true,
		expectedError: nil,
	},
	{
		name:      "DoesNotContainClairPostgresConfiguration",
		component: "clairpostgres",
		managed:   false,
		cfgbundle: map[string][]byte{
			"clair-config.yaml": []byte(``),
		},
		expected:      false,
		expectedError: nil,
	},
}

func TestContainsComponentConfig(t *testing.T) {
	assert := assert.New(t)

	for _, test := range containsComponentConfigTests {
		cmp := v1.Component{Kind: test.component, Managed: test.managed}
		contains, err := ContainsComponentConfig(test.cfgbundle, cmp)
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
		fmt.Errorf("provided certificate pair not valid for host 'registry.company.com': x509: certificate is valid for nonexistent.company.com, not registry.company.com"),
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
		fmt.Errorf("provided certificate pair not valid for host 'builds.company.com': x509: certificate is valid for registry.company.com, not builds.company.com"),
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

			fgn, err := v1.FieldGroupNameFor(v1.ComponentRoute)
			assert.Nil(err, "error not nil: %s", err)

			shared.ValidateCertPairWithHostname(tlsCert, tlsKey, test.serverHostname, fgn)

			if test.buildManagerHostname != "" {
				shared.ValidateCertPairWithHostname(tlsCert, tlsKey, test.buildManagerHostname, fgn)
			}
		}
	}
}

func TestClairMarshal(t *testing.T) {
	tt := []struct {
		_forcekeys struct{}
		Name       string
		In         *config.Config
		Want       string
	}{
		{
			Name: "Zero",
			In:   &config.Config{},
			Want: "clair_zero.yaml",
		},
	}

	sys := os.DirFS("testdata")
	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := yaml.Marshal(tc.In)
			if err != nil {
				t.Error(err)
			}
			want, err := fs.ReadFile(sys, tc.Want)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("got: %+q, want: %+q", got, want)
			}
		})
	}
}
