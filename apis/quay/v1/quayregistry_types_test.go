package v1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ensureDefaultComponentsTests = []struct {
	name        string
	quay        QuayRegistry
	expected    []Component
	expectedErr error
}{
	{
		"AllComponentsProvided",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{SupportsObjectStorageAnnotation: "true"},
			},
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsProvidedWithoutObjectBucketClaims",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		errors.New("cannot use `objectstorage` component when `ObjectBucketClaims` API not available"),
	},
	{
		"AllComponentsProvidedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					SupportsRoutesAnnotation:        "true",
					ClusterHostnameAnnotation:       "apps.example.com",
					SupportsObjectStorageAnnotation: "true",
				},
			},
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsOmitted",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{SupportsObjectStorageAnnotation: "true"},
			},
			Spec: QuayRegistrySpec{},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsOmittedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					SupportsRoutesAnnotation:        "true",
					ClusterHostnameAnnotation:       "apps.example.com",
					SupportsObjectStorageAnnotation: "true",
				},
			},
			Spec: QuayRegistrySpec{},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
	{
		"SomeComponentsProvided",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: false},
					{Kind: "objectstorage", Managed: false},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
	{
		"SomeComponentsProvidedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					SupportsRoutesAnnotation:  "true",
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: false},
					{Kind: "objectstorage", Managed: false},
					{Kind: "route", Managed: false},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: false},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		nil,
	},
}

var canUpgradeTests = []struct {
	name        string
	fromVersion QuayVersion
	expected    bool
}{
	{
		"none",
		"",
		true,
	},
	{
		"nonexistent",
		"not-a-real-version",
		false,
	},
	{
		"current",
		QuayVersionCurrent,
		true,
	},
	{
		"previous",
		QuayVersionPrevious,
		true,
	},
}

func TestCanUpgrade(t *testing.T) {
	assert := assert.New(t)

	for _, test := range canUpgradeTests {
		assert.Equal(test.expected, CanUpgrade(test.fromVersion), test.name)
	}
}

func TestEnsureDefaultComponents(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureDefaultComponentsTests {
		updatedQuay, err := EnsureDefaultComponents(&test.quay)

		if test.expectedErr != nil {
			assert.NotNil(err, test.name)
		} else {
			assert.Nil(err, test.name)
			assert.Equal(len(test.expected), len(updatedQuay.Spec.Components), test.name)

			for _, expectedComponent := range test.expected {
				assert.Contains(updatedQuay.Spec.Components, expectedComponent, test.name)
			}
		}
	}
}

var componentsMatchTests = []struct {
	name             string
	firstComponents  []Component
	secondComponents []Component
	expected         bool
}{
	{
		"EmptyDoNotMatch",
		[]Component{},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "objectstorage", Managed: false},
		},
		false,
	},
	{
		"EmptyMatch",
		[]Component{},
		[]Component{},
		true,
	},
	{
		"Match",
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "objectstorage", Managed: true},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "objectstorage", Managed: true},
		},
		true,
	},
	{
		"DoNotMatch",
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "objectstorage", Managed: true},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "objectstorage", Managed: false},
		},
		false,
	},
}

func TestComponentsMatch(t *testing.T) {
	assert := assert.New(t)

	for _, test := range componentsMatchTests {
		match := ComponentsMatch(test.firstComponents, test.secondComponents)

		assert.Equal(test.expected, match, test.name)
	}
}

var ensureRegistryEndpointTests = []struct {
	name       string
	quay       QuayRegistry
	expected   string
	expectedOk bool
}{
	{
		"SupportsRoutesChanged",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
				Annotations: map[string]string{
					SupportsRoutesAnnotation:  "true",
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
		},
		"test-quay-ns-1.apps.example.com",
		false,
	},
	{
		"SupportsRoutesSame",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
				Annotations: map[string]string{
					SupportsRoutesAnnotation:  "true",
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
			Status: QuayRegistryStatus{
				RegistryEndpoint: "test-quay-ns-1.apps.example.com",
			},
		},
		"test-quay-ns-1.apps.example.com",
		true,
	},
	{
		"DoesNotSupportRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		"",
		true,
	},
}

func TestEnsureRegistryEndpoint(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureRegistryEndpointTests {
		quay, ok := EnsureRegistryEndpoint(&test.quay)

		assert.Equal(test.expectedOk, ok, test.name)
		assert.Equal(test.expected, quay.Status.RegistryEndpoint, test.name)
	}
}
