package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

func TestCreateOrUpdateObject_Job(t *testing.T) {
	jobObj := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{Name: "test", Image: "busybox"},
					},
				},
			},
		},
	}

	for _, tt := range []struct {
		name              string
		existing          bool
		jobStatus         batchv1.JobStatus
		quayVersion       v1.QuayVersion
		createInterceptor func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error
		wantRequeue       bool
		wantErr           bool
		wantJobRecreated  bool
	}{
		{
			name:        "job does not exist, create succeeds",
			existing:    false,
			wantRequeue: false,
			wantErr:     false,
		},
		{
			name:     "job still terminating, returns requeue",
			existing: true,
			createInterceptor: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				return k8serrors.NewAlreadyExists(
					schema.GroupResource{Group: "batch", Resource: "jobs"},
					obj.GetName(),
				)
			},
			wantRequeue: true,
			wantErr:     false,
		},
		{
			name:        "succeeded job skipped when versions match",
			existing:    true,
			jobStatus:   batchv1.JobStatus{Succeeded: 1},
			quayVersion: v1.QuayVersionCurrent,
			wantRequeue: false,
			wantErr:     false,
		},
		{
			name:             "succeeded job recreated during upgrade",
			existing:         true,
			jobStatus:        batchv1.JobStatus{Succeeded: 1},
			quayVersion:      "old-version",
			wantRequeue:      false,
			wantErr:          false,
			wantJobRecreated: true,
		},
		{
			name:        "active job skipped even during upgrade",
			existing:    true,
			jobStatus:   batchv1.JobStatus{Active: 1},
			quayVersion: "old-version",
			wantRequeue: false,
			wantErr:     false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quay := v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
				Status: v1.QuayRegistryStatus{
					CurrentVersion: tt.quayVersion,
				},
			}

			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.existing {
				existingJob := jobObj.DeepCopy()
				existingJob.SetResourceVersion("1")
				existingJob.Status = tt.jobStatus
				builder = builder.WithObjects(existingJob).
					WithStatusSubresource(&batchv1.Job{})
			}
			if tt.createInterceptor != nil {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					Create: tt.createInterceptor,
				})
			}
			fakeClient := builder.Build()

			reconciler := &QuayRegistryReconciler{
				Client:        fakeClient,
				Log:           testLogger,
				Scheme:        scheme.Scheme,
				EventRecorder: testEventRecorder,
			}

			obj := jobObj.DeepCopy()
			requeue, err := reconciler.createOrUpdateObject(
				context.Background(), obj, quay, testLogger,
			)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if requeue != tt.wantRequeue {
				t.Errorf("requeue = %v, want %v", requeue, tt.wantRequeue)
			}

			if tt.wantJobRecreated {
				var got batchv1.Job
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name: jobObj.Name, Namespace: jobObj.Namespace,
				}, &got)
				if err != nil {
					t.Fatalf("expected recreated job to exist: %v", err)
				}
				if got.Status.Succeeded != 0 {
					t.Error("recreated job should not have Succeeded status from old job")
				}
			}
		})
	}
}

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
	_ = v1.EnsureDefaultComponents(
		&quaycontext.QuayRegistryContext{SupportsRoutes: false, SupportsObjectStorage: false, SupportsMonitoring: false},
		quay,
	)

	return quay
}

func newConfigBundle(name, namespace string, withCerts bool) corev1.Secret {
	config := map[string]interface{}{
		"ENTERPRISE_LOGO_URL": "/static/img/quay-horizontal-color.svg",
		"FEATURE_SUPER_USERS": true,
		"SERVER_HOSTNAME":     "quay-app.quay-enterprise",
		// Since the testing cluster doesn't support storage, we must mock an unmanaged storage
		"DISTRIBUTED_STORAGE_CONFIG":            map[string]interface{}{},
		"DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS": []string{},
		"DISTRIBUTED_STORAGE_PREFERENCE":        []string{},
	}

	data := map[string][]byte{
		"config.yaml": encode(config),
	}

	if withCerts {
		cert, key, err := cert.GenerateSelfSignedCertKey(
			config["SERVER_HOSTNAME"].(string),
			nil,
			[]string{config["SERVER_HOSTNAME"].(string)})
		if err != nil {
			panic(err)
		}

		data["ssl.cert"] = cert
		data["ssl.key"] = key
	}

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
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
					Namespace: quayRegistry.GetNamespace(),
				},
				&configBundleSecret)).
				To(Succeed())
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
					Namespace: quayRegistry.GetNamespace(),
				},
				&configBundleSecret)).
				To(Succeed())
		})

		// This test needs to be fixed. Since the ObjectStorage API is not available, we cannot use managed storage.
		// Since the generated config bundle does not have fields for object storage, this secret is never created.
		// In order to properly test this behavior, we must transition our testing into a live cluster (see TODOs)

		// It("should not generate a self-signed TLS cert/key pair in a new `Secret`", func() {
		// 	// Reconcile again to get past defaulting step
		// 	result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})

		// 	var secrets corev1.SecretList
		// 	listOptions := client.ListOptions{
		// 		Namespace: namespace,
		// 		LabelSelector: labels.SelectorFromSet(map[string]string{
		// 			kustomize.QuayRegistryNameLabel: quayRegistryName.Name,
		// 		})}

		// 	Expect(k8sClient.List(context.Background(), &secrets, &listOptions)).To(Succeed())

		// 	found := false
		// 	for _, secret := range secrets.Items {
		// 		if v1.IsManagedTLSSecretFor(quayRegistry, &secret) {
		// 			found = true

		// 			Expect(secret.Data).NotTo(HaveKey("ssl.cert"))
		// 			Expect(secret.Data).NotTo(HaveKey("ssl.key"))
		// 		}
		// 	}

		// 	Expect(found).To(BeTrue())
		// })
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
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace, true)
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
			var updatedQuayRegistry v1.QuayRegistry

			Eventually(func() v1.QuayVersion {
				_ = k8sClient.Get(context.Background(), quayRegistryName, &updatedQuayRegistry)
				return updatedQuayRegistry.Status.CurrentVersion
			}, time.Second*30).Should(Equal(v1.QuayVersionCurrent))
		})

		It("should copy the provided TLS cert/key pair into a new `Secret`", func() {
			var secrets corev1.SecretList
			listOptions := client.ListOptions{
				Namespace: namespace,
				LabelSelector: labels.SelectorFromSet(map[string]string{
					kustomize.QuayRegistryNameLabel: quayRegistryName.Name,
				}),
			}

			Expect(k8sClient.List(context.Background(), &secrets, &listOptions)).To(Succeed())

			found := false
			for _, secret := range secrets.Items {
				if v1.IsManagedTLSSecretFor(quayRegistry, &secret) {
					found = true

					Expect(secret.Data).To(HaveKey("ssl.cert"))
					Expect(secret.Data["ssl.cert"]).To(Equal(configBundle.Data["ssl.cert"]))
					Expect(secret.Data).To(HaveKey("ssl.key"))
					Expect(secret.Data["ssl.key"]).To(Equal(configBundle.Data["ssl.key"]))
				}
			}

			Expect(found).To(BeTrue())
		})
	})

	When("the current version in the `status` block is the same as the Operator", func() {
		BeforeEach(func() {
			quayRegistry = newQuayRegistry("test-registry", namespace)
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace, true)
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
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace, true)
			quayRegistry.Spec.ConfigBundleSecret = configBundle.GetName()
			quayRegistryName = types.NamespacedName{
				Name:      quayRegistry.Name,
				Namespace: quayRegistry.Namespace,
			}

			Expect(k8sClient.Create(context.Background(), &configBundle)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), quayRegistry)).Should(Succeed())

			Expect(k8sClient.Status().Update(context.Background(), quayRegistry)).To(Succeed())

			result, err = controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: quayRegistryName})
		})

		It("successfully performs an upgrade", func() {
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
			configBundle = newConfigBundle("quay-config-secret-abc123", namespace, true)
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

			expectedQuay := quayRegistry.DeepCopy()
			err := v1.EnsureDefaultComponents(
				&quaycontext.QuayRegistryContext{SupportsRoutes: false, SupportsObjectStorage: false, SupportsMonitoring: false},
				expectedQuay,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(v1.ComponentsMatch(updatedQuayRegistry.Spec.Components, expectedQuay.Spec.Components))
		})
	})
})

func quayWithUnmanagedComponents(unmanaged ...v1.ComponentKind) v1.QuayRegistry {
	cmps := make([]v1.Component, len(v1.AllComponents))
	for i, kind := range v1.AllComponents {
		cmp := v1.Component{
			Kind:    kind,
			Managed: true,
		}

		for _, unmkind := range unmanaged {
			if kind != unmkind {
				continue
			}
			cmp.Managed = false
			break
		}

		cmps[i] = cmp
	}

	var quay v1.QuayRegistry
	quay.Spec.Components = cmps
	return quay
}

func Test_hasNecessaryConfig(t *testing.T) {
	for _, tt := range []struct {
		name   string
		experr bool
		cfg    map[string][]byte
		quay   v1.QuayRegistry
	}{
		{
			name:   "all managed",
			experr: false,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged postgres without config",
			experr: true,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentPostgres),
		},
		{
			name:   "unmanaged postgres with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("DB_CONNECTION_ARGS: 'a'\nDB_URI: 'b'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentPostgres),
		},
		{
			name:   "unmanaged clair without config",
			experr: false,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentClair),
		},
		{
			name:   "unmanaged clair with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("SECURITY_SCANNER_ENDPOINT: 'test.com'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentClair),
		},
		{
			name:   "managed clair with config",
			experr: true,
			cfg: map[string][]byte{
				"config.yaml": []byte("SECURITY_SCANNER_ENDPOINT: 'test.com'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged redis",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("USER_EVENTS_REDIS: 'redis.addr.io'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentRedis),
		},
		{
			name:   "managed redis",
			experr: true,
			cfg: map[string][]byte{
				"config.yaml": []byte("USER_EVENTS_REDIS: 'redis.addr.io'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged hpa",
			experr: false,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentHPA),
		},
		{
			name:   "unmanaged objectstorage without config",
			experr: true,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentObjectStorage),
		},
		{
			name:   "unmanaged objectstorage with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("DISTRIBUTED_STORAGE_CONFIG: 'bucket'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentObjectStorage),
		},
		{
			name:   "managed objectstorage with config",
			experr: true,
			cfg: map[string][]byte{
				"config.yaml": []byte("DISTRIBUTED_STORAGE_CONFIG: 'bucket'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "managed route with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("SERVER_HOSTNAME: 'registry.io'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged route without config",
			experr: true,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentRoute),
		},
		{
			name:   "unmanaged route with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("SERVER_HOSTNAME: 'registry.io'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentRoute),
		},
		{
			name:   "unmanaged mirror without config",
			experr: false,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentMirror),
		},
		{
			name:   "unmanaged mirror with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("REPO_MIRROR_INTERVAL: '10'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentMirror),
		},
		{
			name:   "managed mirror with config",
			experr: false,
			cfg: map[string][]byte{
				"config.yaml": []byte("REPO_MIRROR_INTERVAL: '10'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged monitoring",
			experr: false,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentMonitoring),
		},
		{
			name:   "managed tls with certs",
			experr: true,
			cfg: map[string][]byte{
				"ssl.key":  []byte("key"),
				"ssl.cert": []byte("cert"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "unmanaged tls with certs",
			experr: false,
			cfg: map[string][]byte{
				"ssl.key":  []byte("key"),
				"ssl.cert": []byte("cert"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentTLS),
		},
		{
			name:   "unmanaged tls without certs",
			experr: true,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentTLS),
		},
		{
			name:   "managed clairpostgres with config",
			experr: true,
			cfg: map[string][]byte{
				"clair-config.yaml": []byte("matcher:\n  connstring: 'test'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "managed clair and managed clairpostgres with config",
			experr: true,
			cfg: map[string][]byte{
				"clair-config.yaml": []byte("matcher:\n  connstring: 'test'"),
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "managed clair with unmanaged clairpostgres with config",
			experr: false,
			cfg: map[string][]byte{
				"clair-config.yaml": []byte("matcher:\n  connstring: 'test'"),
			},
			quay: quayWithUnmanagedComponents(v1.ComponentClairPostgres),
		},
		{
			name:   "managed clair with unmanaged clairpostgres without config",
			experr: true,
			cfg:    map[string][]byte{},
			quay:   quayWithUnmanagedComponents(v1.ComponentClairPostgres),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := QuayRegistryReconciler{}
			err := reconciler.hasNecessaryConfig(tt.quay, tt.cfg)
			if err != nil {
				if tt.experr {
					return
				}
				t.Errorf("unexpected err: %s", err)
				return
			}

			if tt.experr {
				t.Errorf("expecting error but nil returned")
			}
		})
	}
}

func Test_cleanupPreviousSecret(t *testing.T) {
	for _, tt := range []struct {
		name    string
		experr  bool
		secrets corev1.SecretList
		quay    v1.QuayRegistry
	}{
		{
			name:   "no secrets",
			experr: false,
			secrets: corev1.SecretList{
				Items: []corev1.Secret{},
			},
			quay: quayWithUnmanagedComponents(),
		},
		{
			name:   "no secrets with config bundle",
			experr: false,
			secrets: corev1.SecretList{
				Items: []corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "quay-config-secret",
							Namespace: "test",
						},
					},
				},
			},
		},
		{
			name: "multiple secrets",
			secrets: corev1.SecretList{
				Items: []corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "quay-config-secret-abc123",
							Namespace: "test",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "quay-config-secret-def456",
							Namespace: "test",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := QuayRegistryReconciler{
				Client: fake.NewFakeClient(&tt.secrets),
			}
			log := testLogger.WithValues("test", t.Name())

			err := reconciler.cleanupPreviousSecrets(log, context.Background(), &tt.quay, tt.secrets.Items)
			if err != nil {
				if tt.experr {
					return
				}
				t.Errorf("unexpected err: %s", err)
				return
			}

			if tt.experr {
				t.Errorf("expecting error but nil returned")
			}
		})
	}
}

func newTestOBC(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "objectbucket.io",
		Version: "v1alpha1",
		Kind:    "ObjectBucketClaim",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	_ = unstructured.SetNestedField(obj.Object, "Bound", "status", "phase")
	return obj
}

func TestCheckObjectBucketClaimsAvailable_NamespaceScoped(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "quay-ns",
		},
	}

	// OBC exists in a different namespace — should NOT be found.
	wrongNsOBC := newTestOBC("test-quay-datastore", "other-ns")

	// OBC exists in the correct namespace — should be found.
	correctOBC := newTestOBC("test-quay-datastore", "quay-ns")

	for _, tt := range []struct {
		name            string
		objs            []client.Object
		secrets         []client.Object
		configmaps      []client.Object
		wantInitialized bool
	}{
		{
			name:            "OBC in wrong namespace is ignored",
			objs:            []client.Object{wrongNsOBC},
			wantInitialized: false,
		},
		{
			name: "OBC in correct namespace is found",
			objs: []client.Object{correctOBC},
			secrets: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-quay-datastore",
						Namespace: "quay-ns",
					},
					Data: map[string][]byte{
						"AWS_ACCESS_KEY_ID":     []byte("key"),
						"AWS_SECRET_ACCESS_KEY": []byte("secret"),
					},
				},
			},
			configmaps: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-quay-datastore",
						Namespace: "quay-ns",
					},
					Data: map[string]string{
						"BUCKET_NAME": "mybucket",
						"BUCKET_HOST": "s3.example.com",
					},
				},
			},
			wantInitialized: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder()
			builder.WithObjects(tt.objs...)
			if tt.secrets != nil {
				builder.WithObjects(tt.secrets...)
			}
			if tt.configmaps != nil {
				builder.WithObjects(tt.configmaps...)
			}
			cli := builder.Build()

			reconciler := &QuayRegistryReconciler{
				Client: cli,
				Log:    testLogger,
			}

			qctx := &quaycontext.QuayRegistryContext{}
			err := reconciler.checkObjectBucketClaimsAvailable(
				t.Context(), qctx, quay,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if qctx.ObjectStorageInitialized != tt.wantInitialized {
				t.Errorf("ObjectStorageInitialized = %v, want %v",
					qctx.ObjectStorageInitialized, tt.wantInitialized)
			}
		})
	}
}

func TestCheckManagedDatabaseReady(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	for _, tt := range []struct {
		name                string
		components          []v1.Component
		existingDeployments []appsv1.Deployment
		wantDBInit          bool
		wantClairDBInit     bool
	}{
		{
			name: "both databases unmanaged",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: false},
				{Kind: v1.ComponentClairPostgres, Managed: false},
			},
			wantDBInit:      true,
			wantClairDBInit: true,
		},
		{
			name: "managed postgres not found",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
				{Kind: v1.ComponentClairPostgres, Managed: false},
			},
			wantDBInit:      false,
			wantClairDBInit: true,
		},
		{
			name: "managed postgres zero available replicas",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
				{Kind: v1.ComponentClairPostgres, Managed: false},
			},
			existingDeployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-quay-database",
						Namespace: "default",
					},
					Status: appsv1.DeploymentStatus{AvailableReplicas: 0},
				},
			},
			wantDBInit:      false,
			wantClairDBInit: true,
		},
		{
			name: "managed postgres available",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
				{Kind: v1.ComponentClairPostgres, Managed: false},
			},
			existingDeployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-quay-database",
						Namespace: "default",
					},
					Status: appsv1.DeploymentStatus{AvailableReplicas: 1},
				},
			},
			wantDBInit:      true,
			wantClairDBInit: true,
		},
		{
			name: "managed clair-postgres not found",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: false},
				{Kind: v1.ComponentClairPostgres, Managed: true},
			},
			wantDBInit:      true,
			wantClairDBInit: false,
		},
		{
			name: "managed clair-postgres available",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: false},
				{Kind: v1.ComponentClairPostgres, Managed: true},
			},
			existingDeployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-clair-postgres",
						Namespace: "default",
					},
					Status: appsv1.DeploymentStatus{AvailableReplicas: 1},
				},
			},
			wantDBInit:      true,
			wantClairDBInit: true,
		},
		{
			name: "both managed and available",
			components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
				{Kind: v1.ComponentClairPostgres, Managed: true},
			},
			existingDeployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-quay-database",
						Namespace: "default",
					},
					Status: appsv1.DeploymentStatus{AvailableReplicas: 1},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-clair-postgres",
						Namespace: "default",
					},
					Status: appsv1.DeploymentStatus{AvailableReplicas: 1},
				},
			},
			wantDBInit:      true,
			wantClairDBInit: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quayCopy := quay.DeepCopy()
			quayCopy.Spec.Components = tt.components

			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			for i := range tt.existingDeployments {
				builder = builder.WithObjects(&tt.existingDeployments[i])
			}
			fakeClient := builder.Build()

			reconciler := &QuayRegistryReconciler{
				Client: fakeClient,
				Log:    testLogger,
			}

			qctx := quaycontext.NewQuayRegistryContext()
			reconciler.checkManagedDatabaseReady(context.Background(), qctx, quayCopy)

			if qctx.DatabaseInitialized != tt.wantDBInit {
				t.Errorf("DatabaseInitialized = %v, want %v", qctx.DatabaseInitialized, tt.wantDBInit)
			}
			if qctx.ClairDatabaseInitialized != tt.wantClairDBInit {
				t.Errorf("ClairDatabaseInitialized = %v, want %v", qctx.ClairDatabaseInitialized, tt.wantClairDBInit)
			}
		})
	}
}

func testObj(kind string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Kind: kind})
	return u
}

func testObjWithLabel(kind, component string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Kind: kind})
	u.SetLabels(map[string]string{"quay-component": component})
	return u
}

func TestFilterDeferredWorkloads(t *testing.T) {
	type obj struct {
		kind      string
		component string // quay-component label value
	}
	for _, tt := range []struct {
		name           string
		deferWorkloads bool
		input          []obj
		wantKinds      []string
	}{
		{
			name:           "deferWorkloads false passes all objects through",
			deferWorkloads: false,
			input: []obj{
				{"Secret", ""},
				{"Deployment", "quay-app"},
				{"Job", "quay-app-upgrade"},
				{"Service", ""},
			},
			wantKinds: []string{"Secret", "Deployment", "Job", "Service"},
		},
		{
			name:           "deferWorkloads true filters dependent workloads",
			deferWorkloads: true,
			input: []obj{
				{"Secret", ""},
				{"Deployment", "quay-app"},
				{"Job", "quay-app-upgrade"},
				{"Service", ""},
				{"ConfigMap", ""},
			},
			wantKinds: []string{"Secret", "Service", "ConfigMap"},
		},
		{
			name:           "deferWorkloads true keeps infrastructure deployments",
			deferWorkloads: true,
			input: []obj{
				{"Secret", ""},
				{"Deployment", "postgres"},
				{"Deployment", "clair-postgres"},
				{"Deployment", "redis"},
				{"Deployment", "quay-app"},
				{"Deployment", "clair-app"},
				{"Deployment", "quay-mirror"},
				{"Job", "quay-app-upgrade"},
			},
			wantKinds: []string{"Secret", "Deployment", "Deployment", "Deployment"},
		},
		{
			name:           "deferWorkloads true with no workloads is a no-op",
			deferWorkloads: true,
			input: []obj{
				{"Secret", ""},
				{"Service", ""},
			},
			wantKinds: []string{"Secret", "Service"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.input))
			for i, o := range tt.input {
				if o.component != "" {
					objects[i] = testObjWithLabel(o.kind, o.component)
				} else {
					objects[i] = testObj(o.kind)
				}
			}

			result := filterDeferredWorkloads(objects, tt.deferWorkloads)

			if len(result) != len(tt.wantKinds) {
				t.Fatalf("got %d objects, want %d", len(result), len(tt.wantKinds))
			}
			for i, obj := range result {
				got := obj.GetObjectKind().GroupVersionKind().Kind
				if got != tt.wantKinds[i] {
					t.Errorf("object[%d] kind = %q, want %q", i, got, tt.wantKinds[i])
				}
			}
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }

func TestQuayAppDeploymentRolledOut(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	for _, tt := range []struct {
		name          string
		deployments   []appsv1.Deployment
		wantRolledOut bool
		wantErr       bool
	}{
		{
			name:          "deployment not found returns true (first reconcile)",
			wantRolledOut: true,
		},
		{
			name: "updated replicas less than desired — rollout in progress",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-quay-app", Namespace: "default"},
					Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
					Status:     appsv1.DeploymentStatus{UpdatedReplicas: 1, AvailableReplicas: 2},
				},
			},
			wantRolledOut: false,
		},
		{
			name: "available replicas less than desired — rollout in progress",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-quay-app", Namespace: "default"},
					Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
					Status:     appsv1.DeploymentStatus{UpdatedReplicas: 2, AvailableReplicas: 1},
				},
			},
			wantRolledOut: false,
		},
		{
			name: "all replicas updated and available — rollout complete",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-quay-app", Namespace: "default"},
					Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
					Status:     appsv1.DeploymentStatus{UpdatedReplicas: 2, AvailableReplicas: 2},
				},
			},
			wantRolledOut: true,
		},
		{
			name: "nil Replicas defaults to 1 — rollout complete",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-quay-app", Namespace: "default"},
					Status:     appsv1.DeploymentStatus{UpdatedReplicas: 1, AvailableReplicas: 1},
				},
			},
			wantRolledOut: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			for i := range tt.deployments {
				builder = builder.WithObjects(&tt.deployments[i]).
					WithStatusSubresource(&appsv1.Deployment{})
			}
			cli := builder.Build()

			for i := range tt.deployments {
				st := tt.deployments[i].DeepCopy()
				if err := cli.Status().Update(context.Background(), st); err != nil {
					t.Fatalf("failed to set deployment status: %v", err)
				}
			}

			reconciler := &QuayRegistryReconciler{
				Client: cli,
				Log:    testLogger,
			}

			got, err := reconciler.quayAppDeploymentRolledOut(context.Background(), quay)
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.wantRolledOut {
				t.Errorf("rolledOut = %v, want %v", got, tt.wantRolledOut)
			}
		})
	}
}
