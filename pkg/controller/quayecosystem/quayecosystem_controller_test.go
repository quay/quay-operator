package quayecosystem

import (
	"context"
	"testing"

	"github.com/redhat-cop/operator-utils/pkg/util"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/setup"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var name = "quay-operator"
var namespace = "quay"

func TestDefaultConfiguration(t *testing.T) {
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
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		quayEcosystem,
	}
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(redhatcopv1alpha1.SchemeGroupVersion, quayEcosystem)
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)

	reconcilerBase := util.NewReconcilerBase(cl, s, nil, nil)
	r := &ReconcileQuayEcosystem{reconcilerBase: reconcilerBase, k8sclient: nil, quaySetupManager: setup.NewQuaySetupManager(reconcilerBase, nil)}
	// Initialize the reconicer request
	nsn := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	req := reconcile.Request{
		NamespacedName: nsn,
	}

	r.Reconcile(req)

	// Check if the CRD has been created
	crd := &redhatcopv1alpha1.QuayEcosystem{}
	err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, crd)
	assert.NoError(t, err)
	// Make sure one of the default values was assigned
	assert.Equal(t, crd.Spec.Quay.Image, constants.QuayImage)

}
