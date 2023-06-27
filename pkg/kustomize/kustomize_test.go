package kustomize

import (
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/types"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/middleware"
)

var kustomizationForTests = []struct {
	name         string
	quayRegistry *v1.QuayRegistry
	ctx          quaycontext.QuayRegistryContext
	expected     *types.Kustomization
	expectedErr  string
}{
	{
		"InvalidQuayRegistry",
		nil,
		quaycontext.QuayRegistryContext{},
		nil,
		"given QuayRegistry should not be nil",
	},
	{
		"AllComponents",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Resources: []string{},
			Components: []string{
				"../components/postgres",
				"../components/clair",
				"../components/redis",
				"../components/objectstorage",
				"../components/mirror",
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ComponentImageOverrides",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Resources: []string{},
			Components: []string{
				"../components/postgres",
				"../components/clair",
				"../components/redis",
			},
			Images: []types.Image{
				{Name: "quay.io/projectquay/quay", NewName: "quay", Digest: "sha256:abc123"},
				{Name: "quay.io/projectquay/clair", NewName: "clair", Digest: "sha256:abc123"},
				{Name: "centos/redis-32-centos7", NewName: "redis", Digest: "sha256:abc123"},
				{Name: "centos/postgresql-13-centos7", NewName: "postgres", Digest: "sha256:abc123"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ComponentImageOverridesWithTag",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Resources: []string{},
			Components: []string{
				"../components/postgres",
				"../components/clair",
				"../components/redis",
			},
			Images: []types.Image{
				{Name: "quay.io/projectquay/quay", NewName: "quay", NewTag: "latest"},
				{Name: "quay.io/projectquay/clair", NewName: "clair", NewTag: "alpine"},
				{Name: "centos/redis-32-centos7", NewName: "redis", NewTag: "buster"},
				{Name: "centos/postgresql-13-centos7", NewName: "postgres", NewTag: "latest"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ComponentImageOverridesPostgresUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Resources: []string{},
			Components: []string{
				"../components/clair",
				"../components/redis",
			},
			Images: []types.Image{
				{Name: "quay.io/projectquay/quay", NewName: "quay", NewTag: "latest"},
				{Name: "quay.io/projectquay/clair", NewName: "clair", NewTag: "alpine"},
				{Name: "centos/redis-32-centos7", NewName: "redis", NewTag: "buster"},
				{Name: "centos/postgresql-13-centos7", NewName: "postgres", NewTag: "latest"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ComponentImageOverridesPostgresUpgrade",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			NeedsPgUpgrade: true,
		},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Components: []string{
				"../components/clair",
				"../components/redis",
				"../components/postgres",
				"../components/pgupgrade",
			},
			Images: []types.Image{
				{Name: "quay.io/projectquay/quay", NewName: "quay", NewTag: "latest"},
				{Name: "quay.io/projectquay/clair", NewName: "clair", NewTag: "alpine"},
				{Name: "centos/redis-32-centos7", NewName: "redis", NewTag: "buster"},
				{Name: "centos/postgresql-13-centos7", NewName: "postgres", NewTag: "latest"},
				{Name: "centos/postgresql-10-centos7", NewName: "postgres_previous", NewTag: "latest"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
}

func TestKustomizationFor(t *testing.T) {
	assert := assert.New(t)
	log := logr.Discard()

	for _, test := range kustomizationForTests {
		if test.expected != nil {
			for _, img := range test.expected.Images {
				if len(img.Digest) != 0 {
					os.Setenv("RELATED_IMAGE_COMPONENT_"+strings.ToUpper(img.NewName), img.NewName+"@"+img.Digest)
				} else {
					os.Setenv("RELATED_IMAGE_COMPONENT_"+strings.ToUpper(img.NewName), img.NewName+":"+img.NewTag)
				}
			}
		}

		kustomization, err := KustomizationFor(log, &test.ctx, test.quayRegistry, map[string][]byte{}, "")

		if test.expectedErr != "" {
			assert.EqualError(err, test.expectedErr)
			assert.Nil(kustomization, test.name)
		} else {
			assert.NotNil(kustomization, test.name)

			assert.Equal(len(test.expected.Components), len(kustomization.Components), test.name)
			for _, expectedComponent := range test.expected.Components {
				assert.Contains(kustomization.Components, expectedComponent, test.name)
			}

			assert.Equal(len(test.expected.Images), len(kustomization.Images), test.name)
			for _, img := range test.expected.Images {
				assert.Contains(kustomization.Images, img, test.name)
			}
		}
	}
}

func TestFlattenSecret(t *testing.T) {
	assert := assert.New(t)

	config := map[string]interface{}{
		"ENTERPRISE_LOGO_URL": "/static/img/quay-horizontal-color.svg",
		"FEATURE_SUPER_USERS": true,
		"SERVER_HOSTNAME":     "quay-app.quay-enterprise",
	}

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(config),
			"ssl.key":     encode("abcd1234"),
			"clair.config.yaml": encode(map[string]interface{}{
				"FEATURE_SECURITY_SCANNER":     true,
				"SECURITY_SCANNER_V4_ENDPOINT": "http://quay-clair",
			}),
		},
	}

	flattenedSecret, err := middleware.FlattenSecret(secret)

	assert.Nil(err)
	assert.Equal(2, len(flattenedSecret.Data))
	assert.NotNil(flattenedSecret.Data["config.yaml"])

	flattenedConfig := decode(flattenedSecret.Data["config.yaml"])
	for key, value := range config {
		assert.Equal(value, flattenedConfig.(map[string]interface{})[key])
	}
	assert.Equal(true, flattenedConfig.(map[string]interface{})["FEATURE_SECURITY_SCANNER"])
	assert.Equal("http://quay-clair", flattenedConfig.(map[string]interface{})["SECURITY_SCANNER_V4_ENDPOINT"])
}

var quayComponents = map[string][]client.Object{
	"quay": {
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-editor"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-editor"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-secret"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-tls"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cluster-service-ca"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-editor-credentials"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-registry-managed-secret-keys"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cluster-service-ca"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "extra-ca-certs"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-proxy-config"}},
	},
	"clair": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "clair-config-secret"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "clair-app"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "clair-app"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "clair-app"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres-conf-sample"}},
	},
	"postgres": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "postgres-bootstrap"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "postgres-config-secret"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-database"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "quay-database"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-database"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "quay-database"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "postgres-conf-sample"}},
	},
	"redis": {
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-redis"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-redis"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "quay-redis"}},
	},
	"objectstorage": {
		&objectbucket.ObjectBucketClaim{ObjectMeta: metav1.ObjectMeta{Name: "quay-datastorage"}},
	},
	"route": {
		// TODO: Import OpenShift `Route` API struct
	},
	"mirror": {
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-mirror"}},
	},
	"horizontalpodautoscaler": {
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "quay-mirror"}},
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "clair-app"}},
	},
	"job": {
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "quay-app-upgrade"}},
	},
}

func withComponents(components []string) []client.Object {
	selectedComponents := []client.Object{}
	for _, component := range components {
		selectedComponents = append(selectedComponents, quayComponents[component]...)
	}

	return selectedComponents
}

// TODO(alecmerdler): Test image overrides...
var inflateTests = []struct {
	name         string
	quayRegistry *v1.QuayRegistry
	ctx          quaycontext.QuayRegistryContext
	configBundle *corev1.Secret
	expected     []client.Object
	expectedErr  error
}{
	{
		name: "AllComponentsManagedExplicit",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"job", "quay", "clair", "postgres", "redis", "objectstorage", "mirror", "horizontalpodautoscaler", "clairpostgres"}),
		expectedErr: nil,
	},
	{
		name: "AllComponentsUnmanaged",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
					{Kind: "clair", Managed: false},
					{Kind: "clairpostgres", Managed: false},
					{Kind: "redis", Managed: false},
					{Kind: "objectstorage", Managed: false},
					{Kind: "mirror", Managed: false},
					{Kind: "horizontalpodautoscaler", Managed: false},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"quay"}),
		expectedErr: nil,
	},
	{
		name: "SomeComponentsUnmanaged",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: false},
					{Kind: "objectstorage", Managed: false},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"job", "quay", "postgres", "clair", "mirror", "clairpostgres"}),
		expectedErr: nil,
	},
	{
		name: "CurrentVersion",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
			Status: v1.QuayRegistryStatus{
				CurrentVersion: v1.QuayVersionCurrent,
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			DbUri:                 "postgresql://user:pass@db:5432/db",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"quay", "clair", "postgres", "redis", "objectstorage", "mirror", "clairpostgres"}),
		expectedErr: nil,
	},
	{
		name: "ManagedKeysInProvidedConfig",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
			Status: v1.QuayRegistryStatus{
				CurrentVersion: v1.QuayVersionCurrent,
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			DbUri:                 "postgresql://user:pass@db:5432/db",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io", "DATABASE_SECRET_KEY": "abc123"}),
			},
		},
		expected:    withComponents([]string{"quay", "clair", "postgres", "redis", "objectstorage", "mirror", "clairpostgres"}),
		expectedErr: nil,
	},
	{
		name: "PostgresManagedDbUriExists",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			DbUri: "postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"quay", "postgres"}),
		expectedErr: nil,
	},
	{
		name: "PostgresUnmanagedDbUriExists",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{
					"SERVER_HOSTNAME": "quay.io",
					"DB_URI":          "postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database",
				}),
			},
		},
		expected:    withComponents([]string{"job", "quay"}),
		expectedErr: nil,
	},
	{
		name: "PostgresConfigurationManuallyUpdated",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
				},
			},
			Status: v1.QuayRegistryStatus{
				CurrentVersion: v1.QuayVersionCurrent,
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			DbUri: "postgresql://olduser:oldpass@olddb:5432/olddatabase",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{
					"SERVER_HOSTNAME": "quay.io",
					"DB_URI":          "postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database",
				}),
			},
		},
		expected:    withComponents([]string{"job", "quay"}),
		expectedErr: nil,
	},
	{
		name: "NoChangeInDatabaseButUpgradedVersion",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
				},
			},
			Status: v1.QuayRegistryStatus{
				CurrentVersion: "v0.0.0",
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			DbUri: "postgresql://olduser:oldpass@olddb:5432/olddatabase",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{
					"SERVER_HOSTNAME": "quay.io",
					"DB_URI":          "postgresql://olduser:oldpass@olddb:5432/olddatabase",
				}),
			},
		},
		expected:    withComponents([]string{"job", "quay"}),
		expectedErr: nil,
	},
	{
		name: "RerenderWithoutChanges",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			DbUri:                 "postgresql://user:pass@db:5432/db",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(
					map[string]interface{}{
						"SERVER_HOSTNAME": "quay.io",
						"DB_URI":          "postgresql://user:pass@db:5432/db",
					},
				),
			},
		},
		expected:    withComponents([]string{"quay", "clair", "postgres", "redis", "objectstorage", "mirror", "horizontalpodautoscaler", "clairpostgres"}),
		expectedErr: nil,
	},
	{
		name: "RerenderWithDatabaseChanges",
		quayRegistry: &v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			DbUri:                 "postgresql://user:pass@db:5432/db",
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(
					map[string]interface{}{
						"SERVER_HOSTNAME": "quay.io",
						"DB_URI":          "postgresql://new:new@new:5432/db",
					},
				),
			},
		},
		expected:    withComponents([]string{"job", "quay", "clair", "postgres", "redis", "objectstorage", "mirror", "horizontalpodautoscaler", "clairpostgres"}),
		expectedErr: nil,
	},
}

func TestInflate(t *testing.T) {
	assert := assert.New(t)

	log := testlogr.NewTestLogger(t)

	for _, test := range inflateTests {
		t.Run(test.name, func(t *testing.T) {
			pieces, err := Inflate(&test.ctx, test.quayRegistry, test.configBundle, log, false)

			assert.NotNil(pieces, test.name)
			assert.Equal(len(test.expected), len(pieces), test.name)
			assert.Nil(err, test.name)

			var config map[string]interface{}
			for _, obj := range pieces {
				objectMeta, _ := meta.Accessor(obj)

				if strings.Contains(objectMeta.GetName(), configSecretPrefix) {
					configBundle := obj.(*corev1.Secret)
					config = decode(configBundle.Data["config.yaml"]).(map[string]interface{})
				}
			}

			for _, obj := range pieces {
				objectMeta, _ := meta.Accessor(obj)

				assert.Contains(objectMeta.GetName(), test.quayRegistry.GetName()+"-", test.name)

				if strings.Contains(objectMeta.GetName(), v1.ManagedKeysSecretNameFor(test.quayRegistry)) {
					managedKeys := obj.(*corev1.Secret)

					if test.ctx.DatabaseSecretKey == "" {
						assert.Greater(len(string(managedKeys.Data["DATABASE_SECRET_KEY"])), 0, test.name)
						assert.Greater(len(config["DATABASE_SECRET_KEY"].(string)), 0, test.name)
					} else {
						assert.Equal(test.ctx.DatabaseSecretKey, string(managedKeys.Data["DATABASE_SECRET_KEY"]), test.name)
						assert.Equal(test.ctx.DatabaseSecretKey, config["DATABASE_SECRET_KEY"], test.name)
					}
					assert.Equal(string(managedKeys.Data["DATABASE_SECRET_KEY"]), config["DATABASE_SECRET_KEY"], test.name)

					if test.ctx.SecretKey == "" {
						assert.Greater(len(string(managedKeys.Data["SECRET_KEY"])), 0, test.name)
						assert.Greater(len(config["SECRET_KEY"].(string)), 0, test.name)
					} else {
						assert.Equal(test.ctx.SecretKey, string(managedKeys.Data["SECRET_KEY"]), test.name)
						assert.Equal(test.ctx.SecretKey, config["SECRET_KEY"], test.name)
					}
					assert.Equal(string(managedKeys.Data["SECRET_KEY"]), config["SECRET_KEY"], test.name)

					if test.ctx.DbUri == "" && v1.ComponentIsManaged(test.quayRegistry.Spec.Components, v1.ComponentPostgres) {
						assert.Greater(len(string(managedKeys.Data["DB_URI"])), 0, test.name)
						assert.Greater(len(config["DB_URI"].(string)), 0, test.name)
					} else {
						assert.Equal(test.ctx.DbUri, string(managedKeys.Data["DB_URI"]), test.name)
						assert.Equal(test.ctx.DbUri, config["DB_URI"], test.name)
					}
					assert.Equal(string(managedKeys.Data["DB_URI"]), config["DB_URI"], test.name)
				}
			}
		})
	}
}
