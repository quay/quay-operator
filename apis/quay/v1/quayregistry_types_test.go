package v1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	quaycontext "github.com/quay/quay-operator/pkg/context"
)

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

var ensureDefaultComponentsTests = []struct {
	name        string
	quay        QuayRegistry
	ctx         quaycontext.QuayRegistryContext
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
					{Kind: "objectstorage", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
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
		quaycontext.QuayRegistryContext{},
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
		quaycontext.QuayRegistryContext{
			SupportsRoutes:        true,
			ClusterHostname:       "apps.example.com",
			SupportsObjectStorage: true,
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
			Spec: QuayRegistrySpec{},
		},
		quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
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
			Spec: QuayRegistrySpec{},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:        true,
			ClusterHostname:       "apps.example.com",
			SupportsObjectStorage: true,
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
		quaycontext.QuayRegistryContext{},
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
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: false},
					{Kind: "objectstorage", Managed: false},
					{Kind: "route", Managed: false},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
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

func TestEnsureDefaultComponents(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureDefaultComponentsTests {
		updatedQuay, err := EnsureDefaultComponents(&test.ctx, &test.quay)

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
	ctx        quaycontext.QuayRegistryContext
	config     map[string]interface{}
	expected   string
	expectedOk bool
}{
	{
		"SupportsRoutesChanged",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
		},
		nil,
		"https://test-quay-ns-1.apps.example.com",
		false,
	},
	{
		"SupportsRoutesSame",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
			Status: QuayRegistryStatus{
				RegistryEndpoint: "https://test-quay-ns-1.apps.example.com",
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
		},
		map[string]interface{}{},
		"https://test-quay-ns-1.apps.example.com",
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
		quaycontext.QuayRegistryContext{},
		map[string]interface{}{},
		"",
		true,
	},
	{
		"ServerHostnameInConfigChanged",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		quaycontext.QuayRegistryContext{},
		map[string]interface{}{
			"SERVER_HOSTNAME": "registry.example.com",
		},
		"https://registry.example.com",
		false,
	},
	{
		"ServerHostnameInConfigSame",
		QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
			Status: QuayRegistryStatus{
				RegistryEndpoint: "https://registry.example.com",
			},
		},
		quaycontext.QuayRegistryContext{},
		map[string]interface{}{
			"SERVER_HOSTNAME": "registry.example.com",
		},
		"https://registry.example.com",
		true,
	},
}

func TestEnsureRegistryEndpoint(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureRegistryEndpointTests {
		quay, ok := EnsureRegistryEndpoint(&test.ctx, &test.quay, test.config)

		assert.Equal(test.expectedOk, ok, test.name)
		assert.Equal(test.expected, quay.Status.RegistryEndpoint, test.name)
	}
}
