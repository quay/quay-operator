package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	redhatcop "github.com/quay/quay-operator/apis/redhatcop/v1alpha1"
)

func newQuayEcosystem(name, namespace string) redhatcop.QuayEcosystem {
	return redhatcop.QuayEcosystem{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redhatcop.redhat.io/v1alpha1",
			Kind:       "QuayEcosystem",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: redhatcop.QuayEcosystemSpec{},
	}
}

func randIdentifier(randomBytes int) string {
	identBytes := make([]byte, randomBytes)
	rand.Read(identBytes) // nolint:gosec,errcheck

	return hex.EncodeToString(identBytes)
}

var _ = Describe("QuayEcosystemReconciler", func() {
	var controller *QuayEcosystemReconciler

	var namespace string
	var quayEcosystemName types.NamespacedName
	var quayEcosystem redhatcop.QuayEcosystem

	BeforeEach(func() {
		namespace = randIdentifier(16)
		quayEcosystem := newQuayEcosystem("test-registry", namespace)
		quayEcosystemName = types.NamespacedName{
			Name:      quayEcosystem.Name,
			Namespace: quayEcosystem.Namespace,
		}

		controller = &QuayEcosystemReconciler{
			Client: k8sClient,
			Log:    testLogger,
			Scheme: scheme.Scheme,
		}
	})

	// FIXME(alecmerdler): Disabled because `metadata.namespace` not being set on `QuayEcosystem` for some reason...
	XDescribe("Running Reconcile()", func() {
		var result reconcile.Result
		var err error

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), &quayEcosystem)).Should(Succeed())

			result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
		})

		Context("on a `QuayEcosystem` without the migration label", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should not create a `QuayRegistry`", func() {
				// TODO(alecmerdler)
			})
		})
	})
})
