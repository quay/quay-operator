package kustomize

import (
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/types"

	v1 "github.com/quay/quay-operator/api/v1"
)

var kustomizationForTests = []struct {
	name         string
	quayRegistry *v1.QuayRegistry
	expected     *types.Kustomization
	expectedErr  string
}{
	{
		"InvalidQuayRegistry",
		nil,
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
					{Kind: "storage", Managed: true},
				},
			},
		},
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
				"../components/storage",
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"InvalidDesiredVersion",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: "not-a-real-version",
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "storage", Managed: true},
				},
			},
		},
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
				"../components/storage",
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
	{
		"ValidDesiredVersion",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: v1.QuayVersionPadme,
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "storage", Managed: true},
				},
			},
		},
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
				"../components/storage",
			},
			SecretGenerator: []types.SecretArgs{},
		},
		"",
	},
}

func TestKustomizationFor(t *testing.T) {
	assert := assert.New(t)

	for _, test := range kustomizationForTests {
		kustomization, err := KustomizationFor(test.quayRegistry, map[string][]byte{})

		if test.expectedErr != "" {
			assert.EqualError(err, test.expectedErr)
			assert.Nil(kustomization, test.name)
		} else {
			assert.NotNil(kustomization, test.name)

			assert.Equal(len(test.expected.Components), len(kustomization.Components), test.name)
			for _, expectedComponent := range test.expected.Components {
				assert.Contains(kustomization.Components, expectedComponent, test.name)
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

	flattenedSecret, err := flattenSecret(secret)

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

var quayComponents = map[string][]runtime.Object{
	"base": {
		&rbac.Role{ObjectMeta: metav1.ObjectMeta{Name: "quay-serviceaccount"}},
		&rbac.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "quay-secret-writer"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-app-deployment"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-app"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-config-secret"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-registry-managed-secret-keys"}},
	},
	"clair": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "clair-config-secret"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "clair"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "clair"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "clair-postgres"}},
	},
	"postgres": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "postgres-bootstrap"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-postgres"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "quay-postgres"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-postgres"}},
	},
	"redis": {
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-redis"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-redis"}},
	},
	"storage": {
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "quay-storage"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "quay-datastore"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "quay-datastore"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "minio-pv-claim"}},
	},
	"route": {
		// TODO(alecmerdler): Import OpenShift `Route` API struct
	},
}

func withComponents(components []string) []runtime.Object {
	selectedComponents := []runtime.Object{}
	for _, component := range components {
		selectedComponents = append(selectedComponents, quayComponents[component]...)
	}

	return selectedComponents
}

var inflateTests = []struct {
	name         string
	quayRegistry *v1.QuayRegistry
	configBundle *corev1.Secret
	expected     []runtime.Object
	expectedErr  error
}{
	{
		"AllComponentsManagedExplicit",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: v1.QuayVersionPadme,
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "storage", Managed: true},
				},
			},
		},
		&corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		withComponents([]string{"base", "clair", "postgres", "redis", "storage"}),
		nil,
	},
	{
		"AllComponentsUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: v1.QuayVersionPadme,
				Components: []v1.Component{
					{Kind: "postgres", Managed: false},
					{Kind: "clair", Managed: false},
					{Kind: "redis", Managed: false},
					{Kind: "storage", Managed: false},
				},
			},
		},
		&corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		withComponents([]string{"base"}),
		nil,
	},
	{
		"SomeComponentsUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: v1.QuayVersionPadme,
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: false},
					{Kind: "storage", Managed: false},
				},
			},
		},
		&corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		withComponents([]string{"base", "postgres", "clair"}),
		nil,
	},
	{
		"DesiredVersion",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				DesiredVersion: v1.QuayVersionQuiGon,
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "storage", Managed: true},
				},
			},
		},
		&corev1.Secret{
			Data: map[string][]byte{
				"config.yaml": encode(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"}),
			},
		},
		withComponents([]string{"base", "postgres", "clair", "redis", "storage"}),
		nil,
	},
}

func TestInflate(t *testing.T) {
	assert := assert.New(t)

	log := testlogr.TestLogger{}

	for _, test := range inflateTests {
		pieces, err := Inflate(test.quayRegistry, test.configBundle, nil, log)

		assert.NotNil(pieces, test.name)
		assert.Equal(len(test.expected), len(pieces), test.name)
		assert.Nil(err, test.name)

		for _, obj := range pieces {
			objectMeta, _ := meta.Accessor(obj)

			assert.Contains(objectMeta.GetName(), test.quayRegistry.GetName()+"-", test.name)
			if !strings.Contains(objectMeta.GetName(), "managed-secret-keys") {
				assert.Equal(string(test.quayRegistry.Spec.DesiredVersion), objectMeta.GetAnnotations()["quay-version"], test.name)
			}
		}
	}
}
