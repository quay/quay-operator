package validation

import (
	"testing"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDefaultValidation(t *testing.T) {

	// Objects to track in the fake client.
	objs := []runtime.Object{}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	validate, err := Validate(cl, &quayConfiguration)

	assert.NoError(t, err)
	assert.Equal(t, validate, true)

}
