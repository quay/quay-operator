package unittest

import (
	"testing"

	"github.com/redhat-cop/quay-operator/pkg/apis"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var name = "quay-operator"
var operator = "quay-operator"

func AddToFrameworkSchemeForTests(t *testing.T, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	assert.NoError(t, err)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{
		TypeMeta: metav1.TypeMeta{
			Kind:       "QuayEcosystem",
			APIVersion: "redhatcop.redhat.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: redhatcopv1alpha1.QuayEcosystemSpec{},
		Status: redhatcopv1alpha1.QuayEcosystemStatus{},
	}

	assert.NoError(t, framework.AddToFrameworkScheme(apis.AddToScheme, quayEcosystem))
}
