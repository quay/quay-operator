package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/quay/quay-operator/api/v1"
)

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}

func decode(bytes []byte) interface{} {
	var value interface{}
	_ = yaml.Unmarshal(bytes, &value)

	return value
}

func newQuayRegistry(name, namespace string) v1.QuayRegistry {
	return v1.QuayRegistry{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "quay.redhat.com/v1",
			Kind:       "QuayRegistry",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.QuayRegistrySpec{
			ManagedComponents: []v1.ManagedComponent{
				{Kind: "postgres"},
				{Kind: "clair"},
				{Kind: "redis"},
				{Kind: "storage"},
			},
		},
	}
}

func newConfigBundle(name, namespace string) corev1.Secret {
	config := map[string]interface{}{
		"ENTERPRISE_LOGO_URL": "/static/img/quay-horizontal-color.svg",
		"FEATURE_SUPER_USERS": true,
		"SERVER_HOSTNAME":     "quay-app.quay-enterprise",
	}

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"config.yaml": encode(config),
		},
	}
}

func randIdentifier(randomBytes int) string {
	identBytes := make([]byte, randomBytes)
	rand.Read(identBytes) // nolint:gosec,errcheck

	return hex.EncodeToString(identBytes)
}

var _ = Describe("QuayRegistryReconciler", func() {
	var controller *QuayRegistryReconciler

	var namespace string
	var quayRegistry v1.QuayRegistry
	var quayRegistryName types.NamespacedName
	var configBundle corev1.Secret

	verifyOwnerRefs := func(refs []metav1.OwnerReference) {
		Expect(refs).To(HaveLen(1))
		Expect(refs[0].Kind).To(Equal("QuayRegistry"))
		Expect(refs[0].Name).To(Equal(quayRegistry.GetName()))
	}

	BeforeEach(func() {
		namespace = randIdentifier(16)
		configBundle = newConfigBundle("quay-config-secret-abc123", namespace)
		quayRegistry = newQuayRegistry("test-registry", namespace)
		quayRegistryName = types.NamespacedName{
			Name:      quayRegistry.Name,
			Namespace: quayRegistry.Namespace,
		}
		quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()

		controller = &QuayRegistryReconciler{
			Client: k8sClient,
			Log:    testLogger,
			Scheme: scheme.Scheme,
		}
	})

	Describe("Running Reconcile()", func() {
		var result reconcile.Result
		var err error

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), &quayRegistry)).NotTo(HaveOccurred())
			Expect(k8sClient.Create(context.Background(), &configBundle)).NotTo(HaveOccurred())

			result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
		})

		Context("on a newly created `QuayRegistry`", func() {
			Context("which references a `configBundleSecret` that does not exist", func() {
				BeforeEach(func() {
					quayRegistry.Spec.ConfigBundleSecret = "does-not-exist"
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("will not create any Quay objects on the cluster", func() {
					var deployments appsv1.DeploymentList
					var services corev1.ServiceList
					var persistentVolumeClaims corev1.PersistentVolumeClaimList
					listOptions := client.ListOptions{Namespace: namespace}

					Expect(k8sClient.List(context.Background(), &deployments, &listOptions)).NotTo(HaveOccurred())
					Expect(deployments.Items).To(HaveLen(0))
					Expect(k8sClient.List(context.Background(), &services, &listOptions)).NotTo(HaveOccurred())
					Expect(services.Items).To(HaveLen(0))
					Expect(k8sClient.List(context.Background(), &persistentVolumeClaims, &listOptions)).NotTo(HaveOccurred())
					Expect(persistentVolumeClaims.Items).To(HaveLen(0))
				})
			})

			Context("which references a `configBundleSecret` that does exist", func() {
				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("will create Quay objects on the cluster with `ownerReferences` back to the `QuayRegistry`", func() {
					var deployments appsv1.DeploymentList
					var services corev1.ServiceList
					var persistentVolumeClaims corev1.PersistentVolumeClaimList
					listOptions := client.ListOptions{Namespace: namespace}

					Expect(k8sClient.List(context.Background(), &deployments, &listOptions)).NotTo(HaveOccurred())
					Expect(deployments.Items).NotTo(HaveLen(0))
					for _, deployment := range deployments.Items {
						verifyOwnerRefs(deployment.GetOwnerReferences())
					}
					Expect(k8sClient.List(context.Background(), &services, &listOptions)).NotTo(HaveOccurred())
					Expect(services.Items).NotTo(HaveLen(0))
					for _, service := range services.Items {
						verifyOwnerRefs(service.GetOwnerReferences())
					}
					Expect(k8sClient.List(context.Background(), &persistentVolumeClaims, &listOptions)).NotTo(HaveOccurred())
					Expect(persistentVolumeClaims.Items).NotTo(HaveLen(0))
					for _, persistentVolumeClaim := range persistentVolumeClaims.Items {
						verifyOwnerRefs(persistentVolumeClaim.GetOwnerReferences())
					}
				})
			})
		})

		Context("on an existing `QuayRegistry`", func() {
			var oldPods corev1.PodList
			listOptions := client.ListOptions{Namespace: namespace}

			JustBeforeEach(func() {
				_ = k8sClient.List(context.Background(), &oldPods, &listOptions)
			})

			Context("which references a `configBundleSecret` that does not exist", func() {
				JustBeforeEach(func() {
					quayRegistry.Spec.ConfigBundleSecret = "does-not-exist"
					_ = k8sClient.Update(context.Background(), &quayRegistry)

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("will not update any Quay objects on the cluster", func() {
					var pods corev1.PodList
					listOptions := client.ListOptions{Namespace: namespace}

					_ = k8sClient.List(context.Background(), &pods, &listOptions)
					Expect(len(pods.Items)).To(Equal(len(oldPods.Items)))
					for _, pod := range pods.Items {
						for _, oldPod := range oldPods.Items {
							if pod.GetName() == oldPod.GetName() {
								Expect(pod.GetResourceVersion()).To(Equal(oldPod.GetResourceVersion()))
							}
						}
					}
				})
			})

			Context("which references a `configBundleSecret` that does exist", func() {
				JustBeforeEach(func() {
					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("will update Quay objects on the cluster with `ownerReferences` back to the `QuayRegistry`", func() {
					var deployments appsv1.DeploymentList
					var services corev1.ServiceList
					var persistentVolumeClaims corev1.PersistentVolumeClaimList
					listOptions := client.ListOptions{Namespace: namespace}

					Expect(k8sClient.List(context.Background(), &deployments, &listOptions)).NotTo(HaveOccurred())
					Expect(deployments.Items).NotTo(HaveLen(0))
					for _, deployment := range deployments.Items {
						verifyOwnerRefs(deployment.GetOwnerReferences())
					}
					Expect(k8sClient.List(context.Background(), &services, &listOptions)).NotTo(HaveOccurred())
					Expect(services.Items).NotTo(HaveLen(0))
					for _, service := range services.Items {
						verifyOwnerRefs(service.GetOwnerReferences())
					}
					Expect(k8sClient.List(context.Background(), &persistentVolumeClaims, &listOptions)).NotTo(HaveOccurred())
					Expect(persistentVolumeClaims.Items).NotTo(HaveLen(0))
					for _, persistentVolumeClaim := range persistentVolumeClaims.Items {
						verifyOwnerRefs(persistentVolumeClaim.GetOwnerReferences())
					}
				})
			})
		})

		Context("on a deleted `QuayRegistry`", func() {
			JustBeforeEach(func() {
				_ = k8sClient.Delete(context.Background(), &quayRegistry)
				result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
