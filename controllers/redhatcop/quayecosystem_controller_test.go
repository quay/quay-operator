package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	redhatcop "github.com/quay/quay-operator/apis/redhatcop/v1alpha1"
)

func newQuayEcosystem(name, namespace string) *redhatcop.QuayEcosystem {
	return &redhatcop.QuayEcosystem{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redhatcop.redhat.io/v1alpha1",
			Kind:       "QuayEcosystem",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: redhatcop.QuayEcosystemSpec{
			Quay: &redhatcop.Quay{
				Database:         &redhatcop.Database{VolumeSize: "10Gi", CredentialsSecretName: "quay-database"},
				RegistryBackends: []redhatcop.RegistryBackend{{Name: "default"}},
				ExternalAccess: &redhatcop.ExternalAccess{
					Type: redhatcop.RouteExternalAccessType,
					TLS:  &redhatcop.TLSExternalAccess{Termination: redhatcop.PassthroughTLSTerminationType},
				},
			},
			Clair: &redhatcop.Clair{Enabled: true},
			Redis: &redhatcop.Redis{},
		},
	}
}

func randIdentifier(randomBytes int) string {
	identBytes := make([]byte, randomBytes)
	rand.Read(identBytes) // nolint:gosec,errcheck

	return hex.EncodeToString(identBytes)
}

// TODO: Test suite takes ~2 minutes to complete.
var _ = Describe("Reconciling a QuayEcosystem", func() {
	var controller *QuayEcosystemReconciler

	var namespace string
	var quayEcosystemName types.NamespacedName
	var quayEcosystem *redhatcop.QuayEcosystem
	var quayEnterpriseConfigSecret *corev1.Secret
	var result reconcile.Result
	var err error

	BeforeEach(func() {
		namespace = randIdentifier(16)

		controller = &QuayEcosystemReconciler{
			Client: k8sClient,
			Log:    testLogger,
			Scheme: scheme.Scheme,
		}

		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
	})

	When("it does not have the migration label", func() {
		BeforeEach(func() {
			quayEcosystem = newQuayEcosystem("test-registry", namespace)
			quayEcosystemName = types.NamespacedName{
				Name:      quayEcosystem.Name,
				Namespace: quayEcosystem.Namespace,
			}
			quayEnterpriseConfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quayEnterpriseConfigSecretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{"config.yaml": []byte("")},
			}

			Expect(k8sClient.Create(context.Background(), quayEnterpriseConfigSecret)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

			result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})

		It("should not create a `QuayRegistry`", func() {
			err = k8sClient.Get(context.Background(), quayEcosystemName, &v1.QuayRegistry{})

			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue(), err.Error())
		})
	})

	When("it has the migration label", func() {
		var quayRegistryName types.NamespacedName

		BeforeEach(func() {
			quayEcosystem = newQuayEcosystem("test-registry", namespace)
			quayEcosystemName = types.NamespacedName{
				Name:      quayEcosystem.Name,
				Namespace: quayEcosystem.Namespace,
			}
			quayRegistryName = quayEcosystemName
			quayEcosystem.SetLabels(map[string]string{migrateLabel: "true"})

			quayEnterpriseConfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quayEnterpriseConfigSecretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{"config.yaml": []byte("")},
			}

			Expect(k8sClient.Create(context.Background(), quayEnterpriseConfigSecret)).Should(Succeed())
		})

		When("the config `Secret` does not exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Delete(context.Background(), quayEnterpriseConfigSecret)).Should(Succeed())
				Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

				result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
			})

			It("does not attempt migration", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())

				err = k8sClient.Get(context.Background(), quayRegistryName, &v1.QuayRegistry{})

				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the database component", func() {
			var migrationDeploymentName types.NamespacedName

			BeforeEach(func() {
				migrationDeploymentName = types.NamespacedName{Namespace: namespace, Name: quayEcosystem.GetName() + "-quay-postgres-migration"}
			})

			Context("is unsupported for migration", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Quay.Database = &redhatcop.Database{Server: "some-external-database"}

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("does not create data migration `Deployment`", func() {
					err = k8sClient.Get(context.Background(), migrationDeploymentName, &appsv1.Deployment{})

					Expect(err).To(HaveOccurred())
					Expect(errors.IsNotFound(err)).To(BeTrue())
				})

				It("marks `postgres` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).To(ContainElement(v1.Component{Kind: "postgres", Managed: false}))
				})
			})

			Context("is missing the credentials `Secret` containing the root password", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Quay.Database = &redhatcop.Database{VolumeSize: "10Gi"}

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not create a `QuayRegistry`", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					err = k8sClient.Get(context.Background(), quayRegistryName, &v1.QuayRegistry{})

					Expect(err).To(HaveOccurred())
					Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			})

			Context("is supported for migration", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				// FIXME: Logs show `Deployment` being created, but this fails...
				XIt("creates data migration `Deployment`", func() {
					var migrationDeployment appsv1.Deployment

					Eventually(k8sClient.Get(context.Background(), migrationDeploymentName, &migrationDeployment)).Should(Succeed())
				})

				It("marks `postgres` component as managed", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).NotTo(ContainElement(v1.Component{Kind: "postgres", Managed: false}))
				})
			})
		})

		When("the external access component", func() {
			Context("is unsupported for migration", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Quay.ExternalAccess = &redhatcop.ExternalAccess{Type: redhatcop.LoadBalancerExternalAccessType}

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `route` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).To(ContainElement(v1.Component{Kind: "route", Managed: false}))
				})
			})

			Context("is supported for migration", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `route` component as managed", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).NotTo(ContainElement(v1.Component{Kind: "route", Managed: false}))
				})
			})
		})

		When("the security scanner component", func() {
			Context("is unsupported for migration", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Clair = nil

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `clair` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).To(ContainElement(v1.Component{Kind: "clair", Managed: false}))
				})
			})

			Context("is supported for migration", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `clair` component as managed", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).NotTo(ContainElement(v1.Component{Kind: "clair", Managed: false}))
				})
			})
		})

		When("the object storage component", func() {
			Context("is unsupported for migration", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Quay.RegistryBackends = []redhatcop.RegistryBackend{
						{Name: "local_us", RegistryBackendSource: redhatcop.RegistryBackendSource{Local: &redhatcop.LocalRegistryBackendSource{}}},
					}

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("does not create a `QuayRegistry`", func() {
					err = k8sClient.Get(context.Background(), quayRegistryName, &v1.QuayRegistry{})

					Expect(err).To(HaveOccurred())
					Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			})

			Context("is supported for migration", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `objectstorage` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).To(ContainElement(v1.Component{Kind: "objectstorage", Managed: false}))
				})
			})
		})

		When("the Redis component", func() {
			Context("is unsupported for migration", func() {
				JustBeforeEach(func() {
					quayEcosystem.Spec.Redis = &redhatcop.Redis{Hostname: "my-redis"}

					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `redis` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).To(ContainElement(v1.Component{Kind: "redis", Managed: false}))
				})
			})

			Context("is supported for migration", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(context.Background(), quayEcosystem)).Should(Succeed())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayEcosystemName})
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("marks `redis` component as unmanaged", func() {
					var quayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry)).Should(Succeed())
					Expect(quayRegistry.Spec.Components).NotTo(ContainElement(v1.Component{Kind: "redis", Managed: false}))
				})
			})
		})
	})
})
