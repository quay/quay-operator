package v1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	quaycontext "github.com/quay/quay-operator/pkg/context"
)

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
					{Kind: "clairpostgres", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsObjectStorage: true,
			SupportsMonitoring:    true,
			SupportsRoutes:        true,
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: true},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
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
					{Kind: "clairpostgres", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
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
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: true},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
		},
		errors.New("cannot use `objectstorage` component when `ObjectBucketClaims` API not available"),
	},
	{
		"AllComponentsProvidedWithoutRoutes",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:        false,
			SupportsObjectStorage: true,
			SupportsMonitoring:    true,
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: true},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
		},
		errors.New("cannot use `route` component when `Route` API not available"),
	},
	{
		"TLSComponentProvidedWithoutRoutes",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "clairpostgres", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: false},
					{Kind: "tls", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:        false,
			SupportsObjectStorage: true,
			SupportsMonitoring:    true,
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: true},
			{Kind: "route", Managed: false},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
		},
		errors.New("cannot use `tls` component when `Route` API not available or TLS cert/key pair is provided"),
	},
	{
		"AllComponentsOmitted",
		QuayRegistry{
			Spec: QuayRegistrySpec{},
		},
		quaycontext.QuayRegistryContext{},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: false},
			{Kind: "tls", Managed: false},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: false},
		},
		nil,
	},
	{
		"AllComponentsOmittedWithRoutes",
		QuayRegistry{
			Spec: QuayRegistrySpec{},
		},
		quaycontext.QuayRegistryContext{
			SupportsRoutes:     true,
			ClusterHostname:    "apps.example.com",
			SupportsMonitoring: true,
		},
		[]Component{
			{Kind: "postgres", Managed: true},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: true},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
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
					{Kind: "monitoring", Managed: false},
				},
			},
		},
		quaycontext.QuayRegistryContext{},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: false},
			{Kind: "tls", Managed: false},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: false},
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
			SupportsRoutes:     true,
			SupportsMonitoring: true,
			ClusterHostname:    "apps.example.com",
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: false},
			{Kind: "tls", Managed: true},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
		},
		nil,
	},
	{
		"SomeComponentsProvidedWithTLS",
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
			SupportsRoutes:     true,
			TLSCert:            []byte("my-own-cert"),
			TLSKey:             []byte("my-own-key"),
			SupportsMonitoring: true,
			ClusterHostname:    "apps.example.com",
		},
		[]Component{
			{Kind: "postgres", Managed: false},
			{Kind: "redis", Managed: true},
			{Kind: "clair", Managed: true},
			{Kind: "clairpostgres", Managed: true},
			{Kind: "objectstorage", Managed: false},
			{Kind: "route", Managed: false},
			{Kind: "tls", Managed: false},
			{Kind: "horizontalpodautoscaler", Managed: true},
			{Kind: "mirror", Managed: true},
			{Kind: "monitoring", Managed: true},
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

var validateOverridesTests = []struct {
	name        string
	quay        QuayRegistry
	expectedErr error
}{
	{
		"NoOverridesProvided",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		nil,
	},
	{
		"InvalidVolumeSizeOverride",
		QuayRegistry{
			Spec: QuayRegistrySpec{
				Components: []Component{
					{Kind: "postgres", Managed: true},
					{Kind: "redis", Managed: true},
					{Kind: "clair", Managed: true},
					{Kind: "objectstorage", Managed: true},
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true, Overrides: &Override{VolumeSize: &resource.Quantity{}}},
					{Kind: "horizontalpodautoscaler", Managed: true},
					{Kind: "mirror", Managed: true},
					{Kind: "monitoring", Managed: true},
				},
			},
		},
		errors.New("component tls does not support volumeSize overrides"),
	},
}

func TestValidOverrides(t *testing.T) {
	assert := assert.New(t)

	for _, test := range validateOverridesTests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateOverrides(&test.quay)
			if test.expectedErr != nil {
				assert.NotNil(err, test.name)
				assert.Equal(test.expectedErr, err)
			} else {
				assert.Equal(test.expectedErr, err)
			}
		})
	}
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
		name: "SupportsRoutesChanged",
		quay: QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
		},
		config:     nil,
		expected:   "https://test-quay-ns-1.apps.example.com",
		expectedOk: false,
	},
	{
		name: "SupportsRoutesSame",
		quay: QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
			Status: QuayRegistryStatus{
				RegistryEndpoint: "https://test-quay-ns-1.apps.example.com",
			},
		},
		ctx: quaycontext.QuayRegistryContext{
			SupportsRoutes:  true,
			ClusterHostname: "apps.example.com",
		},
		config:     map[string]interface{}{},
		expected:   "https://test-quay-ns-1.apps.example.com",
		expectedOk: true,
	},
	{
		name: "DoesNotSupportRoutes",
		quay: QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		ctx:        quaycontext.QuayRegistryContext{},
		config:     map[string]interface{}{},
		expected:   "",
		expectedOk: true,
	},
	{
		name: "ServerHostnameInConfigChanged",
		quay: QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
		},
		ctx: quaycontext.QuayRegistryContext{},
		config: map[string]interface{}{
			"SERVER_HOSTNAME": "registry.example.com",
		},
		expected:   "https://registry.example.com",
		expectedOk: false,
	},
	{
		name: "ServerHostnameInConfigSame",
		quay: QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns-1",
			},
			Status: QuayRegistryStatus{
				RegistryEndpoint: "https://registry.example.com",
			},
		},
		ctx: quaycontext.QuayRegistryContext{},
		config: map[string]interface{}{
			"SERVER_HOSTNAME": "registry.example.com",
		},
		expected:   "https://registry.example.com",
		expectedOk: true,
	},
}

func TestEnsureRegistryEndpoint(t *testing.T) {
	assert := assert.New(t)

	for _, test := range ensureRegistryEndpointTests {
		ok := EnsureRegistryEndpoint(&test.ctx, &test.quay, test.config)

		assert.Equal(test.expectedOk, ok, test.name)
		assert.Equal(test.expected, test.quay.Status.RegistryEndpoint, test.name)
	}
}
