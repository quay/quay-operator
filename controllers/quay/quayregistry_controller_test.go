package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

func newQuayRegistry(name, namespace string) *v1.QuayRegistry {
	quay := &v1.QuayRegistry{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "quay.redhat.com/v1",
			Kind:       "QuayRegistry",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.QuayRegistrySpec{},
	}
	// TODO: Test omitting components and marking some as disabled/unmanaged...
	quay, _ = v1.EnsureDefaultComponents(
		&quaycontext.QuayRegistryContext{SupportsRoutes: false, SupportsObjectStorage: false},
		quay.DeepCopy(),
	)

	return quay
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

var _ = Describe("Reconciling a QuayRegistry", func() {
	var controller *QuayRegistryReconciler

	var namespace string
	var quayRegistry *v1.QuayRegistry
	var quayRegistryName types.NamespacedName
	var configBundle corev1.Secret
	var result reconcile.Result
	var err error

	verifyOwnerRefs := func(refs []metav1.OwnerReference) {
		Expect(refs).To(HaveLen(1))
		Expect(refs[0].Kind).To(Equal("QuayRegistry"))
		Expect(refs[0].Name).To(Equal(quayRegistry.GetName()))
	}

	// progressUpgradeDeployment sets the `status` manually because `envtest` only runs apiserver, not controllers.
	progressUpgradeJob := func() error {
		var upgradeJob batchv1.Job
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: quayRegistry.GetName() + "-quay-app-upgrade", Namespace: namespace}, &upgradeJob)
		if err != nil {
			return nil
		}

		upgradeJob.Status.Succeeded = 1

		return k8sClient.Status().Update(context.Background(), &upgradeJob)
	}

	BeforeEach(func() {
		namespace = randIdentifier(16)

		controller = &QuayRegistryReconciler{
			Client:        k8sClient,
			Log:           testLogger,
			Scheme:        scheme.Scheme,
			EventRecorder: testEventRecorder,
		}

		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).Should(Succeed())
	})

	When("the `configBundleSecret` field is empty", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
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

		It("will reference the same `configBundleSecret` when reconciled again", func() {
			var updatedQuayRegistry v1.QuayRegistry
			var configBundleSecretName string
			var configBundleSecret corev1.Secret

			Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).To(Succeed())

			configBundleSecretName = updatedQuayRegistry.Spec.ConfigBundleSecret

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).To(Succeed())
			Expect(updatedQuayRegistry.Spec.ConfigBundleSecret).To(Equal(configBundleSecretName))
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
			quayRegistry = newQuayRegistry("test-registry", namespace)
			quayRegistry.Spec.ConfigBundleSecret = "does-not-exist"
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
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
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace)
			quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("does not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})

		It("will ensure all created objects have `ownerReference` back to the `QuayRegistry`", func() {
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
			Expect(progressUpgradeJob()).To(Succeed())

			var updatedQuayRegistry v1.QuayRegistry

			Eventually(func() v1.QuayVersion {
				_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
				return updatedQuayRegistry.Status.CurrentVersion
			}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
		})
	})

	When("the current version in the `status` block is the same as the Operator", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace)
			quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			quayRegistry.Status.CurrentVersion = v1.QuayVersionCurrent

			Expect(k8sClient.Status().Update(context.Background(), quayRegistry)).To(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("does not attempt an upgrade", func() {
			var updatedQuayRegistry v1.QuayRegistry

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).Should(Succeed())
			Expect(updatedQuayRegistry.Status.CurrentVersion).To(Equal(v1.QuayVersionCurrent))
		})
	})

	When("the current version in the `status` block is upgradable", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace)
			quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			quayRegistry.Status.CurrentVersion = v1.QuayVersionPrevious

			Expect(k8sClient.Status().Update(context.Background(), quayRegistry)).To(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("successfully performs an upgrade", func() {
			Expect(progressUpgradeJob()).To(Succeed())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			var updatedQuayRegistry v1.QuayRegistry

			Eventually(func() v1.QuayVersion {
				_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
				return updatedQuayRegistry.Status.CurrentVersion
			}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
		})
	})

	When("the current version in the `status` block is not upgradable", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			quayRegistry.Status.CurrentVersion = "not-a-real-version"

			Expect(k8sClient.Status().Update(context.Background(), quayRegistry)).To(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("does not attempt an upgrade", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			var updatedQuayRegistry v1.QuayRegistry

			Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).Should(Succeed())
			Expect(string(quayRegistry.Status.CurrentVersion)).To(Equal("not-a-real-version"))
		})
	})

	When("not all default managed components have been specified", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace)
			quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()
			quayRegistry.Spec.Components = nil

			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("does not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})

		It("populates `spec.components` field with all default managed components", func() {
			var updatedQuayRegistry v1.QuayRegistry

			Expect(k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)).Should(Succeed())

			expectedQuay, err := v1.EnsureDefaultComponents(
				&quaycontext.QuayRegistryContext{SupportsRoutes: false, SupportsObjectStorage: false},
				quayRegistry.DeepCopy(),
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(v1.ComponentsMatch(updatedQuayRegistry.Spec.Components, expectedQuay.Spec.Components))
		})
	})
})
