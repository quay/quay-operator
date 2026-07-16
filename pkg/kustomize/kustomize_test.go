package kustomize

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
					{Kind: "clairpostgres", Managed: true},
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
				"../components/clairpostgres",
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
				{Name: "quay.io/sclorg/redis-7-c9s", NewName: "redis", Digest: "sha256:abc123"},
				{Name: "quay.io/sclorg/postgresql-13-c9s", NewName: "postgres", Digest: "sha256:abc123"},
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
				{Name: "quay.io/sclorg/redis-7-c9s", NewName: "redis", NewTag: "buster"},
				{Name: "quay.io/sclorg/postgresql-13-c9s", NewName: "postgres", NewTag: "latest"},
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
				{Name: "quay.io/sclorg/redis-7-c9s", NewName: "redis", NewTag: "buster"},
				{Name: "quay.io/sclorg/postgresql-13-c9s", NewName: "postgres", NewTag: "latest"},
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
				{Name: "quay.io/sclorg/redis-7-c9s", NewName: "redis", NewTag: "buster"},
				{Name: "quay.io/sclorg/postgresql-13-c9s", NewName: "postgres", NewTag: "latest"},
				{Name: "centos/postgresql-10-centos7", NewName: "postgres_previous", NewTag: "latest"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ClairPostgresUpgradeUnmanagedClair",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "clairpostgres", Managed: true},
					{Kind: "clair", Managed: false},
					{Kind: "redis", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			NeedsClairPgUpgrade: true,
		},
		&types.Kustomization{
			TypeMeta: types.TypeMeta{
				APIVersion: types.KustomizationVersion,
				Kind:       types.KustomizationKind,
			},
			Components: []string{
				"../components/redis",
				"../components/clairpostgres",
				"../components/clairpgupgrade/base",
			},
			Images: []types.Image{
				{Name: "quay.io/projectquay/quay", NewName: "quay", NewTag: "latest"},
				{Name: "quay.io/projectquay/clair", NewName: "clair", NewTag: "alpine"},
				{Name: "quay.io/sclorg/redis-7-c9s", NewName: "redis", NewTag: "buster"},
				{Name: "quay.io/sclorg/postgresql-15-c9s", NewName: "clairpostgres", NewTag: "latest"},
				{Name: "quay.io/sclorg/postgresql-13-c9s", NewName: "clairpostgres_previous", NewTag: "latest"},
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
}

func TestEnsureCreationOrder(t *testing.T) {
	objects := []client.Object{
		&appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "quay-app"},
		},
		&corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "quay-config-secret"},
		},
		&batchv1.Job{
			TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "quay-app-upgrade"},
		},
		&corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "quay-app"},
		},
	}

	sorted := EnsureCreationOrder(objects)

	// Deployments and Jobs should come after Secrets and Services
	for i, obj := range sorted {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		if kind == "Deployment" || kind == "Job" {
			// Every remaining object should also be a Deployment or Job
			for _, remaining := range sorted[i:] {
				rKind := remaining.GetObjectKind().GroupVersionKind().Kind
				assert.True(t, rKind == "Deployment" || rKind == "Job",
					"expected Deployment or Job at end, got %s", rKind)
			}
			break
		}
	}

	// First two should be non-Deployment/Job kinds, last two should be Deployment/Job
	for _, obj := range sorted[:2] {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		assert.True(t, kind != "Deployment" && kind != "Job",
			"expected non-Deployment/Job in first half, got %s", kind)
	}
	for _, obj := range sorted[2:] {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		assert.True(t, kind == "Deployment" || kind == "Job",
			"expected Deployment or Job in second half, got %s", kind)
	}
}

func TestKustomizationFor(t *testing.T) {
	assert := assert.New(t)
	log := logr.Discard()

	for _, test := range kustomizationForTests {
		if test.expected != nil {
			for _, img := range test.expected.Images {
				if len(img.Digest) != 0 {
					t.Setenv("RELATED_IMAGE_COMPONENT_"+strings.ToUpper(img.NewName), img.NewName+"@"+img.Digest)
				} else {
					t.Setenv("RELATED_IMAGE_COMPONENT_"+strings.ToUpper(img.NewName), img.NewName+":"+img.NewTag)
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

func TestMonitoringLabelChain(t *testing.T) {
	log := testlogr.NewTestLogger(t)

	ctx := quaycontext.QuayRegistryContext{
		SupportsObjectStorage:    true,
		ObjectStorageInitialized: true,
		SupportsMonitoring:       true,
	}
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true},
				{Kind: "redis", Managed: true},
				{Kind: "objectstorage", Managed: true},
				{Kind: "mirror", Managed: true},
				{Kind: "monitoring", Managed: true},
			},
		},
	}
	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
		},
	}

	pieces, err := Inflate(&ctx, quay, configBundle, log, false)
	require.NoError(t, err)
	require.NotEmpty(t, pieces)

	var (
		metricsService *corev1.Service
		appDep         *appsv1.Deployment
		mirrorDep      *appsv1.Deployment
	)

	for _, obj := range pieces {
		accessor, _ := meta.Accessor(obj)
		name := accessor.GetName()

		switch dep := obj.(type) {
		case *corev1.Service:
			if strings.Contains(name, "quay-metrics") {
				metricsService = dep
			}
		case *appsv1.Deployment:
			if strings.Contains(name, "quay-app") && !strings.Contains(name, "upgrade") {
				appDep = dep
			}
			if strings.Contains(name, "quay-mirror") {
				mirrorDep = dep
			}
		}
	}

	require.NotNil(t, metricsService, "quay-metrics Service should be present")
	assert.Equal(t, "true", metricsService.Spec.Selector["quay-monitor"],
		"quay-metrics Service selector should use quay-monitor label")

	require.NotNil(t, appDep, "quay-app Deployment should be present")
	assert.Equal(t, "true", appDep.Spec.Template.Labels["quay-monitor"],
		"quay-app pod template should have quay-monitor label")

	require.NotNil(t, mirrorDep, "quay-mirror Deployment should be present")
	assert.Equal(t, "true", mirrorDep.Spec.Template.Labels["quay-monitor"],
		"quay-mirror pod template should have quay-monitor label")
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
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-secret"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-tls"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cluster-service-ca"}},
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
		func() *unstructured.Unstructured {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "objectbucket.io",
				Version: "v1alpha1",
				Kind:    "ObjectBucketClaim",
			})
			obj.SetName("quay-datastorage")
			return obj
		}(),
	},
	"route": {
		// TODO: Import OpenShift `Route` API struct
	},
	"mirror": {
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-mirror"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-mirror-pushgateway"}},
	},
	"horizontalpodautoscaler": {
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "quay-mirror"}},
		&autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "clair-app"}},
	},
	"clairpostgres": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "clairpostgres-config-secret"}},
	},
	"monitoring": {
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-metrics"}},
		&rbac.Role{ObjectMeta: metav1.ObjectMeta{Name: "quay-metrics"}},
		&rbac.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "quay-metrics"}},
		func() *unstructured.Unstructured {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
			obj.SetName("quay-metrics-monitor")
			return obj
		}(),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "quay-grafana-dashboard"}},
		&rbac.Role{ObjectMeta: metav1.ObjectMeta{Name: "quay-alerts"}},
		&rbac.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "quay-alerts"}},
		func() *unstructured.Unstructured {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
			obj.SetName("quay-alerts")
			return obj
		}(),
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
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
			SupportsMonitoring:       true,
		},
		configBundle: &corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		expected:    withComponents([]string{"job", "quay", "clair", "postgres", "redis", "objectstorage", "mirror", "horizontalpodautoscaler", "clairpostgres", "monitoring"}),
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
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
			DbUri:                    "postgresql://user:pass@db:5432/db",
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

func TestInflateEmptyConfigYAML(t *testing.T) {
	log := testlogr.NewTestLogger(t)

	tests := []struct {
		name         string
		configBundle *corev1.Secret
	}{
		{
			name: "EmptyConfigYAML",
			configBundle: &corev1.Secret{
				Data: map[string][]byte{
					"config.yaml": {},
				},
			},
		},
		{
			name: "MissingConfigYAMLKey",
			configBundle: &corev1.Secret{
				Data: map[string][]byte{},
			},
		},
		{
			name:         "NilData",
			configBundle: &corev1.Secret{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := quaycontext.QuayRegistryContext{
				SupportsObjectStorage:    true,
				ObjectStorageInitialized: true,
			}
			quay := &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
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
			}

			pieces, err := Inflate(&ctx, quay, tt.configBundle, log, false)
			assert.NoError(t, err, tt.name)
			assert.NotNil(t, pieces, tt.name)
		})
	}
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

func TestInflatePushgatewayURLInjected(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{}

	quayRegistry := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true},
				{Kind: "clair", Managed: false},
				{Kind: "clairpostgres", Managed: false},
				{Kind: "redis", Managed: true},
				{Kind: "objectstorage", Managed: false},
				{Kind: "mirror", Managed: true},
				{Kind: "horizontalpodautoscaler", Managed: false},
			},
		},
	}

	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
		},
	}

	pieces, err := Inflate(&ctx, quayRegistry, configBundle, log, false)
	assert.Nil(t, err)
	assert.NotNil(t, pieces)

	for _, obj := range pieces {
		objectMeta, _ := meta.Accessor(obj)
		if strings.Contains(objectMeta.GetName(), configSecretPrefix) {
			secret := obj.(*corev1.Secret)
			config := decode(secret.Data["config.yaml"]).(map[string]interface{})
			assert.Equal(t, "http://test-quay-mirror-pushgateway:9091", config["PROMETHEUS_PUSHGATEWAY_URL"])
			return
		}
	}
	t.Fatal("config secret not found in inflated objects")
}

func TestInflatePushgatewayURLNotOverridden(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{}

	quayRegistry := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true},
				{Kind: "clair", Managed: false},
				{Kind: "clairpostgres", Managed: false},
				{Kind: "redis", Managed: true},
				{Kind: "objectstorage", Managed: false},
				{Kind: "mirror", Managed: true},
				{Kind: "horizontalpodautoscaler", Managed: false},
			},
		},
	}

	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{
				"SERVER_HOSTNAME":            "quay.io",
				"PROMETHEUS_PUSHGATEWAY_URL": "http://custom-pushgateway:9091",
			}),
		},
	}

	pieces, err := Inflate(&ctx, quayRegistry, configBundle, log, false)
	assert.Nil(t, err)
	assert.NotNil(t, pieces)

	for _, obj := range pieces {
		objectMeta, _ := meta.Accessor(obj)
		if strings.Contains(objectMeta.GetName(), configSecretPrefix) {
			secret := obj.(*corev1.Secret)
			config := decode(secret.Data["config.yaml"]).(map[string]interface{})
			assert.Equal(t, "http://custom-pushgateway:9091", config["PROMETHEUS_PUSHGATEWAY_URL"])
			return
		}
	}
	t.Fatal("config secret not found in inflated objects")
}

func TestProgrammaticBootstrapEnabledStrictBool(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config map[string]interface{}
		want   bool
	}{
		{
			name:   "boolean true enables programmatic bootstrap",
			config: map[string]interface{}{ProgrammaticBootstrapFeatureConfigField: true},
			want:   true,
		},
		{
			name:   "string true does not enable programmatic bootstrap",
			config: map[string]interface{}{ProgrammaticBootstrapFeatureConfigField: "true"},
			want:   false,
		},
		{
			name:   "nil value does not enable programmatic bootstrap",
			config: map[string]interface{}{ProgrammaticBootstrapFeatureConfigField: nil},
			want:   false,
		},
		{
			name:   "missing key does not enable programmatic bootstrap",
			config: map[string]interface{}{},
			want:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ProgrammaticBootstrapEnabled(tt.config))
		})
	}
}

func TestInflateProgrammaticBootstrapTokenEnabled(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{
		DbUri: "postgresql://user:pass@db:5432/db",
	}
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec:       v1.QuayRegistrySpec{},
		Status:     v1.QuayRegistryStatus{CurrentVersion: v1.QuayVersionCurrent},
	}
	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{
				"SERVER_HOSTNAME":                       "quay.io",
				"DB_URI":                                ctx.DbUri,
				ProgrammaticBootstrapFeatureConfigField: true,
			}),
		},
	}

	pieces, err := Inflate(&ctx, quay, configBundle, log, false)
	require.NoError(t, err)

	secretName := BootstrapTokenSecretName(quay)
	config := renderedQuayConfig(t, pieces)
	assert.Equal(t, secretName, config[ProgrammaticTokenK8sSecretConfigField])
	assert.Equal(t, BootstrapTokenSecretKey, config[ProgrammaticTokenK8sKeyConfigField])
	assert.Equal(t, BootstrapTokenConfigPath, config[ProgrammaticTokenPathConfigField])
	assert.NotContains(t, config, "PROGRAMMATIC_TOKEN_K8S_NAMESPACE")

	bootstrapSecret := findSecretByName(pieces, secretName)
	require.NotNil(t, bootstrapSecret)
	assert.Equal(t, corev1.SecretTypeOpaque, bootstrapSecret.Type)
	assert.NotContains(t, bootstrapSecret.Data, BootstrapTokenSecretKey)

	role := findRoleByName(pieces, secretName)
	require.NotNil(t, role)
	require.Len(t, role.Rules, 1)
	assert.Equal(t, []string{""}, role.Rules[0].APIGroups)
	assert.Equal(t, []string{"secrets"}, role.Rules[0].Resources)
	assert.Equal(t, []string{secretName}, role.Rules[0].ResourceNames)
	assert.ElementsMatch(t, []string{"get", "update"}, role.Rules[0].Verbs)

	roleBinding := findRoleBindingByName(pieces, secretName)
	require.NotNil(t, roleBinding)
	assert.Equal(t, rbac.GroupName, roleBinding.RoleRef.APIGroup)
	assert.Equal(t, "Role", roleBinding.RoleRef.Kind)
	assert.Equal(t, secretName, roleBinding.RoleRef.Name)
	require.Len(t, roleBinding.Subjects, 1)
	assert.Equal(t, rbac.ServiceAccountKind, roleBinding.Subjects[0].Kind)
	assert.Equal(t, "test-quay-app", roleBinding.Subjects[0].Name)
	assert.Equal(t, "test-ns", roleBinding.Subjects[0].Namespace)

	deployment := findDeploymentByName(pieces, "test-quay-app")
	require.NotNil(t, deployment)
	volume := findVolumeByName(deployment, bootstrapTokenVolumeName)
	require.NotNil(t, volume)
	require.NotNil(t, volume.Secret)
	assert.Equal(t, secretName, volume.Secret.SecretName)

	mount := findVolumeMountByName(deployment, "quay-app", bootstrapTokenVolumeName)
	require.NotNil(t, mount)
	assert.Equal(t, BootstrapTokenMountPath, mount.MountPath)
	assert.True(t, mount.ReadOnly)
	assert.Empty(t, mount.SubPath)
}

func TestInflateProgrammaticBootstrapTokenDisabled(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{
		DbUri: "postgresql://user:pass@db:5432/db",
	}
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec:       v1.QuayRegistrySpec{},
		Status:     v1.QuayRegistryStatus{CurrentVersion: v1.QuayVersionCurrent},
	}
	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{
				"SERVER_HOSTNAME":                       "quay.io",
				"DB_URI":                                ctx.DbUri,
				ProgrammaticBootstrapFeatureConfigField: false,
			}),
		},
	}

	pieces, err := Inflate(&ctx, quay, configBundle, log, false)
	require.NoError(t, err)

	secretName := BootstrapTokenSecretName(quay)
	config := renderedQuayConfig(t, pieces)
	assert.NotContains(t, config, ProgrammaticTokenK8sSecretConfigField)
	assert.NotContains(t, config, ProgrammaticTokenK8sKeyConfigField)
	assert.NotContains(t, config, ProgrammaticTokenPathConfigField)

	assert.Nil(t, findSecretByName(pieces, secretName))
	assert.Nil(t, findRoleByName(pieces, secretName))
	assert.Nil(t, findRoleBindingByName(pieces, secretName))

	deployment := findDeploymentByName(pieces, "test-quay-app")
	require.NotNil(t, deployment)
	assert.Nil(t, findVolumeByName(deployment, bootstrapTokenVolumeName))
	assert.Nil(t, findVolumeMountByName(deployment, "quay-app", bootstrapTokenVolumeName))
}

func TestInflateProgrammaticBootstrapTokenOverridesConflictingConfig(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{
		DbUri: "postgresql://user:pass@db:5432/db",
	}
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec:       v1.QuayRegistrySpec{},
		Status:     v1.QuayRegistryStatus{CurrentVersion: v1.QuayVersionCurrent},
	}
	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{
				"SERVER_HOSTNAME":                       "quay.io",
				"DB_URI":                                ctx.DbUri,
				ProgrammaticBootstrapFeatureConfigField: true,
				ProgrammaticTokenK8sSecretConfigField:   "user-secret",
				ProgrammaticTokenK8sKeyConfigField:      "user-key.json",
				ProgrammaticTokenPathConfigField:        "/tmp/user-token.json",
			}),
		},
	}

	pieces, err := Inflate(&ctx, quay, configBundle, log, false)
	require.NoError(t, err)

	config := renderedQuayConfig(t, pieces)
	assert.Equal(t, BootstrapTokenSecretName(quay), config[ProgrammaticTokenK8sSecretConfigField])
	assert.Equal(t, BootstrapTokenSecretKey, config[ProgrammaticTokenK8sKeyConfigField])
	assert.Equal(t, BootstrapTokenConfigPath, config[ProgrammaticTokenPathConfigField])
}

func TestValidateProgrammaticBootstrapConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "feature disabled",
			config:  map[string]interface{}{ProgrammaticBootstrapFeatureConfigField: false},
			wantErr: false,
		},
		{
			name:    "feature absent",
			config:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "feature enabled with owner set",
			config: map[string]interface{}{
				ProgrammaticBootstrapFeatureConfigField: true,
				BootstrapTokenOwnerConfigField:          "admin",
			},
			wantErr: false,
		},
		{
			name: "feature enabled without owner",
			config: map[string]interface{}{
				ProgrammaticBootstrapFeatureConfigField: true,
			},
			wantErr: true,
		},
		{
			name: "feature enabled with empty owner",
			config: map[string]interface{}{
				ProgrammaticBootstrapFeatureConfigField: true,
				BootstrapTokenOwnerConfigField:          "",
			},
			wantErr: true,
		},
		{
			name: "feature enabled with non-string owner",
			config: map[string]interface{}{
				ProgrammaticBootstrapFeatureConfigField: true,
				BootstrapTokenOwnerConfigField:          42,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProgrammaticBootstrapConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInflateProgrammaticBootstrapTokenPreservesUserConfig(t *testing.T) {
	log := testlogr.NewTestLogger(t)
	ctx := quaycontext.QuayRegistryContext{
		DbUri: "postgresql://user:pass@db:5432/db",
	}
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec:       v1.QuayRegistrySpec{},
		Status:     v1.QuayRegistryStatus{CurrentVersion: v1.QuayVersionCurrent},
	}
	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{
				"SERVER_HOSTNAME":                       "quay.io",
				"DB_URI":                                ctx.DbUri,
				ProgrammaticBootstrapFeatureConfigField: true,
				BootstrapTokenOwnerConfigField:          "bootstrap-admin",
				"BOOTSTRAP_TOKEN_EXPIRATION":            3600,
				"BOOTSTRAP_TOKEN_SCOPE":                 "org:admin",
			}),
		},
	}

	pieces, err := Inflate(&ctx, quay, configBundle, log, false)
	require.NoError(t, err)

	config := renderedQuayConfig(t, pieces)
	assert.Equal(t, "bootstrap-admin", config[BootstrapTokenOwnerConfigField])
	assert.Equal(t, 3600, int(config["BOOTSTRAP_TOKEN_EXPIRATION"].(float64)))
	assert.Equal(t, "org:admin", config["BOOTSTRAP_TOKEN_SCOPE"])
}

func renderedQuayConfig(t *testing.T, pieces []client.Object) map[string]interface{} {
	t.Helper()
	for _, obj := range pieces {
		secret, ok := obj.(*corev1.Secret)
		if !ok || !strings.Contains(secret.GetName(), configSecretPrefix) {
			continue
		}
		return decode(secret.Data["config.yaml"]).(map[string]interface{})
	}
	t.Fatal("rendered Quay config Secret not found")
	return nil
}

func findSecretByName(pieces []client.Object, name string) *corev1.Secret {
	for _, obj := range pieces {
		secret, ok := obj.(*corev1.Secret)
		if ok && secret.GetName() == name {
			return secret
		}
	}
	return nil
}

func findRoleByName(pieces []client.Object, name string) *rbac.Role {
	for _, obj := range pieces {
		role, ok := obj.(*rbac.Role)
		if ok && role.GetName() == name {
			return role
		}
	}
	return nil
}

func findRoleBindingByName(pieces []client.Object, name string) *rbac.RoleBinding {
	for _, obj := range pieces {
		roleBinding, ok := obj.(*rbac.RoleBinding)
		if ok && roleBinding.GetName() == name {
			return roleBinding
		}
	}
	return nil
}

func findDeploymentByName(pieces []client.Object, name string) *appsv1.Deployment {
	for _, obj := range pieces {
		deployment, ok := obj.(*appsv1.Deployment)
		if ok && deployment.GetName() == name {
			return deployment
		}
	}
	return nil
}

func findVolumeByName(deployment *appsv1.Deployment, name string) *corev1.Volume {
	for index := range deployment.Spec.Template.Spec.Volumes {
		if deployment.Spec.Template.Spec.Volumes[index].Name == name {
			return &deployment.Spec.Template.Spec.Volumes[index]
		}
	}
	return nil
}

func findVolumeMountByName(deployment *appsv1.Deployment, containerName, name string) *corev1.VolumeMount {
	for containerIndex := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[containerIndex]
		if container.Name != containerName {
			continue
		}
		for mountIndex := range container.VolumeMounts {
			if container.VolumeMounts[mountIndex].Name == name {
				return &container.VolumeMounts[mountIndex]
			}
		}
	}
	return nil
}

func TestInflateTLSCertGeneration(t *testing.T) {
	log := testlogr.NewTestLogger(t)

	newQuay := func(components []v1.Component) *v1.QuayRegistry {
		return &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
			Spec: v1.QuayRegistrySpec{
				Components: components,
			},
		}
	}

	configBundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
		},
	}

	findManagedKeys := func(t *testing.T, pieces []client.Object, quay *v1.QuayRegistry) *corev1.Secret {
		t.Helper()
		for _, obj := range pieces {
			objectMeta, _ := meta.Accessor(obj)
			if strings.Contains(objectMeta.GetName(), v1.ManagedKeysSecretNameFor(quay)) {
				return obj.(*corev1.Secret)
			}
		}
		t.Fatal("managed keys Secret not found in Inflate output")
		return nil
	}

	t.Run("generates certs when TLS enabled on postgres", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		assert.NotEmpty(t, ctx.PostgresTLSCA)
		assert.NotEmpty(t, ctx.PostgresTLSCert)
		assert.NotEmpty(t, ctx.PostgresTLSKey)

		// Verify cert has correct SANs
		block, _ := pem.Decode([]byte(ctx.PostgresTLSCert))
		require.NotNil(t, block)
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		assert.Contains(t, cert.DNSNames, "test-quay-database")
		assert.Contains(t, cert.DNSNames, "test-quay-database.test-ns.svc.cluster.local")

		// Verify certs persisted in managed keys Secret
		managedKeys := findManagedKeys(t, pieces, quay)
		assert.Equal(t, ctx.PostgresTLSCA, string(managedKeys.Data["POSTGRES_TLS_CA"]))
		assert.Equal(t, ctx.PostgresTLSCert, string(managedKeys.Data["POSTGRES_TLS_CERT"]))
		assert.Equal(t, ctx.PostgresTLSKey, string(managedKeys.Data["POSTGRES_TLS_KEY"]))

		// Verify DB_URI contains TLS params
		assert.Contains(t, ctx.DbUri, "sslmode=verify-full")
		assert.Contains(t, ctx.DbUri, "sslrootcert")
		assert.Contains(t, string(managedKeys.Data["DB_URI"]), "sslmode=verify-full")
	})

	t.Run("generates certs when TLS enabled on clairpostgres", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true},
			{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			{Kind: "clair", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		assert.NotEmpty(t, ctx.ClairPostgresTLSCA)
		assert.NotEmpty(t, ctx.ClairPostgresTLSCert)
		assert.NotEmpty(t, ctx.ClairPostgresTLSKey)

		// Verify cert has correct SANs for clair database
		block, _ := pem.Decode([]byte(ctx.ClairPostgresTLSCert))
		require.NotNil(t, block)
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		assert.Contains(t, cert.DNSNames, "test-clair-postgres")
		assert.Contains(t, cert.DNSNames, "test-clair-postgres.test-ns.svc.cluster.local")

		// Verify persisted
		managedKeys := findManagedKeys(t, pieces, quay)
		assert.Equal(t, ctx.ClairPostgresTLSCA, string(managedKeys.Data["CLAIRPOSTGRES_TLS_CA"]))
		assert.Equal(t, ctx.ClairPostgresTLSCert, string(managedKeys.Data["CLAIRPOSTGRES_TLS_CERT"]))
		assert.Equal(t, ctx.ClairPostgresTLSKey, string(managedKeys.Data["CLAIRPOSTGRES_TLS_KEY"]))
	})

	t.Run("does not generate certs when secretRef is set", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
				Enabled:   true,
				SecretRef: &corev1.LocalObjectReference{Name: "user-provided-certs"},
			}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		_, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		assert.Empty(t, ctx.PostgresTLSCA)
		assert.Empty(t, ctx.PostgresTLSCert)
		assert.Empty(t, ctx.PostgresTLSKey)
	})

	t.Run("does not generate certs when TLS not enabled", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		_, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		assert.Empty(t, ctx.PostgresTLSCA)
		assert.Empty(t, ctx.PostgresTLSCert)
		assert.Empty(t, ctx.PostgresTLSKey)
		assert.NotContains(t, ctx.DbUri, "sslmode")
	})

	t.Run("preserves existing certs on re-run (idempotent)", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		// First run: generates certs
		_, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)
		firstCA := ctx.PostgresTLSCA
		firstCert := ctx.PostgresTLSCert
		firstKey := ctx.PostgresTLSKey
		require.NotEmpty(t, firstCA)

		// Second run: should preserve, not regenerate
		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)
		assert.Equal(t, firstCA, ctx.PostgresTLSCA)
		assert.Equal(t, firstCert, ctx.PostgresTLSCert)
		assert.Equal(t, firstKey, ctx.PostgresTLSKey)

		// Managed keys Secret should have the same certs
		managedKeys := findManagedKeys(t, pieces, quay)
		assert.Equal(t, firstCA, string(managedKeys.Data["POSTGRES_TLS_CA"]))
	})

	t.Run("creates postgres-tls and postgresql-ca Secrets in output", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		findSecret := func(nameSuffix string) *corev1.Secret {
			for _, obj := range pieces {
				objectMeta, _ := meta.Accessor(obj)
				if strings.HasSuffix(objectMeta.GetName(), nameSuffix) {
					if s, ok := obj.(*corev1.Secret); ok {
						return s
					}
				}
			}
			return nil
		}

		tlsSecret := findSecret("postgres-tls")
		require.NotNil(t, tlsSecret, "expected postgres-tls Secret in Inflate output")
		assert.NotEmpty(t, tlsSecret.Data["tls.crt"])
		assert.NotEmpty(t, tlsSecret.Data["tls.key"])

		caSecret := findSecret("postgresql-ca")
		require.NotNil(t, caSecret, "expected postgresql-ca Secret in Inflate output")
		assert.NotEmpty(t, caSecret.Data["ca.crt"])
	})

	t.Run("does not create TLS Secrets when secretRef is set", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
				Enabled:   true,
				SecretRef: &corev1.LocalObjectReference{Name: "user-certs"},
			}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		for _, obj := range pieces {
			objectMeta, _ := meta.Accessor(obj)
			name := objectMeta.GetName()
			assert.False(t, strings.HasSuffix(name, "postgres-tls"), "should not create postgres-tls Secret when secretRef is set")
			assert.False(t, strings.HasSuffix(name, "postgresql-ca"), "should not create postgresql-ca Secret when secretRef is set")
		}
	})

	t.Run("skips cert generation when UseServiceCA is true for postgres", func(t *testing.T) {
		ctx := quaycontext.QuayRegistryContext{
			SupportsObjectStorage:    true,
			ObjectStorageInitialized: true,
			SupportsRoutes:           true,
			PostgresUseServiceCA:     true,
			PostgresSSLRootCert:      "/conf/stack/extra_ca_certs/service-ca.crt",
		}
		quay := newQuay([]v1.Component{
			{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			{Kind: "redis", Managed: true},
			{Kind: "objectstorage", Managed: true},
		})

		pieces, err := Inflate(&ctx, quay, configBundle.DeepCopy(), log, true)
		require.NoError(t, err)

		assert.Empty(t, ctx.PostgresTLSCA, "should not generate CA when using service CA")
		assert.Empty(t, ctx.PostgresTLSCert, "should not generate cert when using service CA")
		assert.Empty(t, ctx.PostgresTLSKey, "should not generate key when using service CA")

		for _, obj := range pieces {
			objectMeta, _ := meta.Accessor(obj)
			name := objectMeta.GetName()
			if strings.Contains(name, "postgres-tls") && obj.GetObjectKind().GroupVersionKind().Kind == "Secret" {
				t.Errorf("should not generate postgres-tls secret when using service CA, found %q", name)
			}
			if strings.Contains(name, "postgresql-ca") {
				t.Errorf("should not generate postgresql-ca secret when using service CA, found %q", name)
			}
		}

		assert.Contains(t, ctx.DbUri, "sslmode=verify-full")
		assert.Contains(t, ctx.DbUri, "sslrootcert=%2Fconf%2Fstack%2Fextra_ca_certs%2Fservice-ca.crt")
	})
}
