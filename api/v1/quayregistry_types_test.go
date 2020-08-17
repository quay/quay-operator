package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
