package v1

import (
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
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "localstorage", Managed: true},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsProvidedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "localstorage", Managed: true},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: true},
			{Kind: "route", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsOmitted",
		QuayRegistry{
			Spec: QuayRegistrySpec{},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: true},
		},
		nil,
	},
	{
		"AllComponentsOmittedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
			Spec: QuayRegistrySpec{},
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: true},
			{Kind: "route", Managed: true},
		},
		nil,
	},
	{
		"SomeComponentsProvided",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: false},
					{Kind: "localstorage", Managed: false},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: false},
		},
		nil,
	},
	{
		"SomeComponentsProvidedWithRoutes",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ClusterHostnameAnnotation: "apps.example.com",
				},
			},
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: false},
					{Kind: "localstorage", Managed: false},
					{Kind: "route", Managed: false},
				},
			},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "localstorage", Managed: false},
			{Kind: "route", Managed: false},
		},
		nil,
	},
}

func TestEnsureDefaultComponents(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureDefaultComponentsTests {
		updatedQuay, err := EnsureDefaultComponents(&test.quay)

		assert.Nil(err, test.name)
		assert.Equal(len(test.expected), len(updatedQuay.Spec.Components), test.name)

		for _, expectedComponent := range test.expected {
			assert.Contains(updatedQuay.Spec.Components, expectedComponent, test.name)
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
			{Kind: "localstorage", Managed: false},
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
			{Kind: "localstorage", Managed: true},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "localstorage", Managed: true},
		},
		true,
	},
	{
		"DoNotMatch",
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "localstorage", Managed: true},
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: false},
			{Kind: "localstorage", Managed: false},
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
