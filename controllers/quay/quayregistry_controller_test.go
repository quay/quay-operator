package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
)

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
			Components: []v1.Component{
				// FIXME(alecmerdler): Test omitting components and marking some as disabled/unmanaged...
				{Kind: "postgres", Managed: true},
				{Kind: "clair", Managed: true},
				{Kind: "redis", Managed: true},
				{Kind: "objectstorage", Managed: false},
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

		// progressUpgradeDeployment sets the `status` manually because `envtest` only runs apiserver, not controllers.
		progressUpgradeDeployment := func() error {
			var upgradeDeployment appsv1.Deployment
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: quayRegistry.GetName() + "-quay-app-upgrade", Namespace: namespace}, &upgradeDeployment)
			if err != nil {
				return err
			}

			upgradeDeployment.Status.Replicas = 1
			upgradeDeployment.Status.ReadyReplicas = 1

			return k8sClient.Status().Update(context.Background(), &upgradeDeployment)
		}

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), &quayRegistry)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())

			result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
		})

		JustAfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
		})

		Context("on a newly created `QuayRegistry`", func() {
			When("the `configBundleSecret` field is empty", func() {
				BeforeEach(func() {
					quayRegistry.Spec.ConfigBundleSecret = ""
				})

				It("should not return an error", func() {
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should create a fresh `Secret` and populate `configBundleSecret`", func() {
					var updatedQuayRegistry v1.QuayRegistry
					var configBundleSecret corev1.Secret

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).To(Succeed())
					Expect(updatedQuayRegistry.Spec.ConfigBundleSecret).To(ContainSubstring(quayRegistry.GetName() + "-config-bundle-"))

					Expect(k8sClient.Get(
						context.Background(),
						types.NamespacedName{
							Name:      updatedQuayRegistry.Spec.ConfigBundleSecret,
							Namespace: quayRegistry.GetNamespace()},
						&configBundleSecret)).
						Should(Succeed())
				})
			})

			When("it references a `configBundleSecret` that does not exist", func() {
				BeforeEach(func() {
					quayRegistry.Spec.ConfigBundleSecret = "does-not-exist"
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
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

				It("does not set the current version in the `status` block", func() {
					var updatedQuayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).Should(Succeed())
					Expect(len(updatedQuayRegistry.Status.CurrentVersion)).To(Equal(0))
				})
			})

			When("it references a `configBundleSecret` that does exist", func() {
				JustBeforeEach(func() {
					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				// FIXME(alecmerdler): This test is failing because there are zero `Deployments` being created...
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

				// FIXME(alecmerdler): This test is failing because there are zero `Deployments` being created...
				It("reports the current version in the `status` block", func() {
					Expect(progressUpgradeDeployment()).Should(Succeed())

					var updatedQuayRegistry v1.QuayRegistry

					Eventually(func() v1.QuayVersion {
						_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
						return updatedQuayRegistry.Status.CurrentVersion
					}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
				})

				When("the `spec.components` field is empty", func() {
					It("will add all backing components as managed", func() {

					})
				})
			})

			When("running on native Kubernetes", func() {
				XIt("reports `Service` load balancer endpoint in the `status` block", func() {
					Expect(progressUpgradeDeployment()).Should(Succeed())

					var updatedQuayRegistry v1.QuayRegistry

					Eventually(func() string {
						_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
						return updatedQuayRegistry.Status.RegistryEndpoint
					}, time.Second*30).Should(Equal("16.22.171.225"))
				})
			})

			When("running on OpenShift", func() {
				XIt("reports registry `Route` endpoint in the `status` block", func() {
					Expect(progressUpgradeDeployment()).Should(Succeed())

					var updatedQuayRegistry v1.QuayRegistry

					Eventually(func() string {
						_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
						return updatedQuayRegistry.Status.RegistryEndpoint
					}, time.Second*30).Should(Equal(strings.Join([]string{
						strings.Join([]string{quayRegistry.GetName(), "quay", quayRegistry.GetNamespace()}, "-"),
						"example.com"},
						".")))
				})
			})
		})

		Context("on an existing `QuayRegistry`", func() {
			var oldPods corev1.PodList
			listOptions := client.ListOptions{Namespace: namespace}

			JustBeforeEach(func() {
				_ = k8sClient.List(context.Background(), &oldPods, &listOptions)
			})

			When("the `configBundleSecret` field is empty", func() {
				BeforeEach(func() {
					quayRegistry.Spec.ConfigBundleSecret = ""
				})

				It("should not return an error", func() {
					Expect(err).ShouldNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should create a fresh `Secret` and populate `configBundleSecret`", func() {
					var updatedQuayRegistry v1.QuayRegistry
					var configBundleSecret corev1.Secret

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).To(Succeed())
					Expect(updatedQuayRegistry.Spec.ConfigBundleSecret).To(ContainSubstring(quayRegistry.GetName() + "-config-bundle-"))

					Expect(k8sClient.Get(
						context.Background(),
						types.NamespacedName{
							Name:      updatedQuayRegistry.Spec.ConfigBundleSecret,
							Namespace: quayRegistry.GetNamespace()},
						&configBundleSecret)).
						Should(Succeed())
				})
			})

			When("it references a `configBundleSecret` that does not exist", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Get(context.Background(), quayRegistryName, &quayRegistry))
					quayRegistry.Spec.ConfigBundleSecret = "does-not-exist"
					Expect(k8sClient.Update(context.Background(), &quayRegistry)).NotTo(HaveOccurred())

					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
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

				It("does not change the current version in the `status` block", func() {
					var updatedQuayRegistry v1.QuayRegistry

					Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry))
					Expect(updatedQuayRegistry.Status.CurrentVersion).To(Equal(quayRegistry.Status.CurrentVersion))
				})
			})

			When("it references a `configBundleSecret` that does exist", func() {
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

				It("reports the current version in the `status` block", func() {
					Expect(progressUpgradeDeployment()).Should(Succeed())

					var updatedQuayRegistry v1.QuayRegistry

					Eventually(func() v1.QuayVersion {
						_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
						return updatedQuayRegistry.Status.CurrentVersion
					}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
				})
			})

			When("the current version in the `status` block is the same as the Operator", func() {
				JustBeforeEach(func() {
					quayRegistry.Status.CurrentVersion = v1.QuayVersionCurrent
					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("does not attempt an upgrade", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(quayRegistry.Status.CurrentVersion).To(Equal(v1.QuayVersionCurrent))
				})
			})

			When("the current version in the `status` block is upgradable", func() {
				JustBeforeEach(func() {
					quayRegistry.Status.CurrentVersion = v1.QuayVersionPrevious
					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("successfully performs an upgrade", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(progressUpgradeDeployment()).Should(Succeed())

					var updatedQuayRegistry v1.QuayRegistry

					Eventually(func() v1.QuayVersion {
						_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
						return updatedQuayRegistry.Status.CurrentVersion
					}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
				})
			})

			When("the current version in the `status` block is not upgradable", func() {
				JustBeforeEach(func() {
					quayRegistry.Status.CurrentVersion = "not-a-real-version"
					result, err = controller.Reconcile(reconcile.Request{NamespacedName: quayRegistryName})
				})

				It("does not attempt an upgrade", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(string(quayRegistry.Status.CurrentVersion)).To(Equal("not-a-real-version"))
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
				Expect(result.Requeue).To(BeFalse())
			})
		})
	})
})
