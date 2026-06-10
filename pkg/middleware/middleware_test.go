package middleware

import (
	"fmt"
	"testing"

	route "github.com/openshift/api/route/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1k8s "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
)

var processTests = []struct {
	name          string
	quay          *v1.QuayRegistry
	obj           client.Object
	expected      client.Object
	expectedError error
}{
	{
		"quayConfigBundle",
		&v1.QuayRegistry{},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "quay-config-bundle",
			},
			Data: map[string][]byte{},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "quay-config-bundle",
			},
			Data: map[string][]byte{},
		},
		nil,
	},
	{
		"quayAppRouteTLSUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: false},
				},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
			Spec: route.RouteSpec{
				Port: &route.RoutePort{
					TargetPort: intstr.Parse("https"),
				},
				TLS: &route.TLSConfig{
					Termination:                   route.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		},
		nil,
	},
	{
		"quayBuilderRouteTLSUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: false},
				},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-builder-route"},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-builder-route"},
			},
			Spec: route.RouteSpec{
				TLS: &route.TLSConfig{
					Termination:                   route.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		},
		nil,
	},
	{
		"quayAppRouteTLSManaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
				},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
		},
		nil,
	},
	{
		"routeAnnotationOverrideTLSManaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true, Overrides: &v1.Override{
						Annotations: map[string]string{
							"haproxy.router.openshift.io/ip_whitelist": "1.2.3.4",
						},
					}},
					{Kind: "tls", Managed: true},
				},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
				Annotations: map[string]string{
					"haproxy.router.openshift.io/ip_whitelist": "1.2.3.4",
				},
			},
		},
		nil,
	},
	{
		"routeLabelOverrideTLSUnmanaged",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true, Overrides: &v1.Override{
						Labels: map[string]string{
							"custom-label": "my-value",
						},
						Annotations: map[string]string{
							"haproxy.router.openshift.io/ip_whitelist": "1.2.3.4",
						},
					}},
					{Kind: "tls", Managed: false},
				},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "quay-app-route"},
			},
		},
		&route.Route{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"quay-component": "quay-app-route",
					"custom-label":   "my-value",
				},
				Annotations: map[string]string{
					"haproxy.router.openshift.io/ip_whitelist": "1.2.3.4",
				},
			},
			Spec: route.RouteSpec{
				Port: &route.RoutePort{
					TargetPort: intstr.Parse("https"),
				},
				TLS: &route.TLSConfig{
					Termination:                   route.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		},
		nil,
	},
	{
		"volumeSizeDefault",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "postgres"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")}},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "postgres"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")}},
			},
		},
		nil,
	},
	{
		"volumeSizeOverridePostgres",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{VolumeSize: parseResourceString("60Gi")}},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "postgres"},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "postgres"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("60Gi")}},
			},
		},
		nil,
	},
	{
		"volumeSizeOverrideClairPostgres",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{VolumeSize: parseResourceString("70Gi")}},
					{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{VolumeSize: parseResourceString("60Gi")}},
					{Kind: "clair", Managed: true, Overrides: &v1.Override{VolumeSize: parseResourceString("60Gi")}},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "clair-postgres"},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "clair-postgres"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("60Gi")}},
			},
		},
		nil,
	},
	{
		"volumeSizeShrinkError",
		&v1.QuayRegistry{
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "route", Managed: true},
					{Kind: "tls", Managed: true},
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{VolumeSize: parseResourceString("30Gi")}},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"quay-component": "postgres"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")}},
			},
		},
		nil,
		fmt.Errorf("cannot shrink volume override size from 50Gi to 30Gi"),
	},
}

func TestProcess(t *testing.T) {
	assert := assert.New(t)
	for _, test := range processTests {

		t.Run(test.name, func(t *testing.T) {
			quayContext := quaycontext.NewQuayRegistryContext()
			processedObj, err := Process(test.quay, quayContext, test.obj, false)
			if test.expectedError != nil {
				assert.Error(err, test.name)
			} else {
				assert.Nil(err, test.name)
			}
			assert.Equal(test.expected, processedObj, test.name)
		})

	}
}

func boolPtr(b bool) *bool { return &b }

func parseResourceString(s string) *resource.Quantity {
	resourceSize := resource.MustParse(s)
	return &resourceSize
}

func newClairDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-clair-app",
			Labels:      map[string]string{"quay-component": "clair-app"},
			Annotations: map[string]string{"quay-component": "clair"},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "clair-app"},
					},
					Volumes: []corev1.Volume{
						{
							Name: "indexer-layer-storage",
							VolumeSource: corev1.VolumeSource{
								Ephemeral: &corev1.EphemeralVolumeSource{
									VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
										Spec: corev1.PersistentVolumeClaimSpec{
											AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceStorage: resource.MustParse("20Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestProcessClairEphemeralVolumeOverrides(t *testing.T) {
	tests := []struct {
		name                 string
		quay                 *v1.QuayRegistry
		expectedStorage      string
		expectedStorageClass *string
	}{
		{
			name: "NoOverrides",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentClair, Managed: true},
					},
				},
			},
			expectedStorage:      "20Gi",
			expectedStorageClass: nil,
		},
		{
			name: "VolumeSizeOverride",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentClair, Managed: true, Overrides: &v1.Override{
							VolumeSize: parseResourceString("50Gi"),
						}},
					},
				},
			},
			expectedStorage:      "50Gi",
			expectedStorageClass: nil,
		},
		{
			name: "StorageClassOverride",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentClair, Managed: true, Overrides: &v1.Override{
							StorageClassName: ptr.To("fast-storage"),
						}},
					},
				},
			},
			expectedStorage:      "20Gi",
			expectedStorageClass: ptr.To("fast-storage"),
		},
		{
			name: "BothOverrides",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentClair, Managed: true, Overrides: &v1.Override{
							VolumeSize:       parseResourceString("100Gi"),
							StorageClassName: ptr.To("premium-storage"),
						}},
					},
				},
			},
			expectedStorage:      "100Gi",
			expectedStorageClass: ptr.To("premium-storage"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := newClairDeployment()
			qctx := quaycontext.NewQuayRegistryContext()

			result, err := Process(tt.quay, qctx, dep, false)
			assert.NoError(t, err)

			processedDep, ok := result.(*appsv1.Deployment)
			assert.True(t, ok)

			var found bool
			for _, vol := range processedDep.Spec.Template.Spec.Volumes {
				if vol.Name != "indexer-layer-storage" {
					continue
				}
				found = true
				vct := vol.Ephemeral.VolumeClaimTemplate.Spec
				actualStorage := vct.Resources.Requests[corev1.ResourceStorage]
				assert.Equal(t, tt.expectedStorage, actualStorage.String(),
					"volume size mismatch")

				if tt.expectedStorageClass == nil {
					assert.Nil(t, vct.StorageClassName, "expected no storageClassName")
				} else {
					assert.NotNil(t, vct.StorageClassName, "expected storageClassName to be set")
					assert.Equal(t, *tt.expectedStorageClass, *vct.StorageClassName)
				}
			}
			assert.True(t, found, "indexer-layer-storage volume not found")
		})
	}
}

func TestHPAWithUnmanagedMirrorAndClair(t *testing.T) {
	quayRegistry := &v1.QuayRegistry{
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "mirror", Managed: false},
				{Kind: "clair", Managed: false},
				{Kind: "horizontalpodautoscaler", Managed: true},
			},
		},
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registry-quay-app",
			Labels: map[string]string{
				"quay-component": "clair",
			},
		},
	}

	// Create a mock context and logger
	qctx := &quaycontext.QuayRegistryContext{}

	// Call the Process function
	result, err := Process(quayRegistry, qctx, hpa, false)

	// Assert that there is no error
	assert.NoError(t, err)

	// Assert that the result is nil
	assert.Nil(t, result)
}

func TestProcessPVCStorageClassNameOverride(t *testing.T) {
	tests := []struct {
		name                   string
		componentKind          v1.ComponentKind
		componentLabel         string
		storageClassName       *string
		expectedStorageClass   *string
		initialPVCStorageClass *string
	}{
		{
			name:                 "Postgres with StorageClassName override",
			componentKind:        v1.ComponentPostgres,
			componentLabel:       "postgres",
			storageClassName:     ptr.To("my-fast-storage"),
			expectedStorageClass: ptr.To("my-fast-storage"),
		},
		{
			name:                 "Postgres without StorageClassName override",
			componentKind:        v1.ComponentPostgres,
			componentLabel:       "postgres",
			storageClassName:     nil,
			expectedStorageClass: nil,
		},
		{
			name:                 "ClairPostgres with StorageClassName override",
			componentKind:        v1.ComponentClairPostgres,
			componentLabel:       "clair-postgres",
			storageClassName:     ptr.To("clair-storage"),
			expectedStorageClass: ptr.To("clair-storage"),
		},
		{
			name:                   "Postgres with initial StorageClassName and no override",
			componentKind:          v1.ComponentPostgres,
			componentLabel:         "postgres",
			storageClassName:       nil,
			initialPVCStorageClass: ptr.To("default-storage"),
			expectedStorageClass:   ptr.To("default-storage"),
		},
		{
			name:                   "Postgres with initial StorageClassName and different override",
			componentKind:          v1.ComponentPostgres,
			componentLabel:         "postgres",
			storageClassName:       ptr.To("override-storage"),
			initialPVCStorageClass: ptr.To("initial-storage"),
			expectedStorageClass:   ptr.To("override-storage"),
		},
		{
			name:                 "Irrelevant component (redis) with override, postgres PVC without",
			componentKind:        v1.ComponentRedis,       // Override set for Redis
			componentLabel:       "postgres",              // PVC is for Postgres
			storageClassName:     ptr.To("redis-storage"), // This should not apply to the postgres PVC
			expectedStorageClass: nil,                     // Postgres PVC should not get redis-storage
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quayReg := &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "testquay", Namespace: "testns"},
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{
							Kind:    tt.componentKind,
							Managed: true,
						},
						// Add postgres component if we are testing redis to ensure the postgres PVC is processed against its own (lack of) override
						{
							Kind:    v1.ComponentPostgres,
							Managed: true,
						},
					},
				},
			}

			// Apply override to the correct component in the QuayRegistry spec
			for i, comp := range quayReg.Spec.Components {
				if comp.Kind == tt.componentKind {
					if comp.Overrides == nil {
						quayReg.Spec.Components[i].Overrides = &v1.Override{}
					}
					quayReg.Spec.Components[i].Overrides.StorageClassName = tt.storageClassName
				}
			}

			// if tt.componentKind is ComponentRedis, and we are testing a postgres PVC, make sure postgres component does not have an override
			if tt.componentKind == v1.ComponentRedis && tt.componentLabel == "postgres" {
				for i, comp := range quayReg.Spec.Components {
					if comp.Kind == v1.ComponentPostgres {
						if comp.Overrides != nil {
							quayReg.Spec.Components[i].Overrides.StorageClassName = nil
						}
					}
				}
			}

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "testns",
					Labels:    map[string]string{"quay-component": tt.componentLabel},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					StorageClassName: tt.initialPVCStorageClass,
				},
			}

			qctx := &quaycontext.QuayRegistryContext{} // Mock context, add fields if needed

			processedObj, err := Process(quayReg, qctx, pvc, false)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			processedPVC, ok := processedObj.(*corev1.PersistentVolumeClaim)
			if !ok {
				t.Fatalf("Processed object is not a PersistentVolumeClaim")
			}

			if tt.expectedStorageClass == nil {
				if processedPVC.Spec.StorageClassName != nil {
					t.Errorf("Expected StorageClassName to be nil, got %v", *processedPVC.Spec.StorageClassName)
				}
			} else {
				if processedPVC.Spec.StorageClassName == nil {
					t.Errorf("Expected StorageClassName to be %v, got nil", *tt.expectedStorageClass)
				} else if *processedPVC.Spec.StorageClassName != *tt.expectedStorageClass {
					t.Errorf("Expected StorageClassName %v, got %v", *tt.expectedStorageClass, *processedPVC.Spec.StorageClassName)
				}
			}
		})
	}
}

func TestProcessDeploymentSecurityContextOverride(t *testing.T) {
	overrideSC := &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(false),
		AllowPrivilegeEscalation: boolPtr(true),
	}

	defaultSC := &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(true),
		AllowPrivilegeEscalation: boolPtr(false),
	}

	tests := []struct {
		name                string
		quay                *v1.QuayRegistry
		dep                 *appsv1.Deployment
		expectedContainerSC *corev1.SecurityContext
		expectedInitSC      *corev1.SecurityContext
	}{
		{
			name: "SecurityContextOverrideReplacesContainers",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentQuay, Managed: true, Overrides: &v1.Override{SecurityContext: overrideSC}},
					},
				},
			},
			dep: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-quay-app",
					Labels:      map[string]string{"quay-component": "quay-app"},
					Annotations: map[string]string{"quay-component": "quay"},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "quay-app", SecurityContext: defaultSC},
							},
							InitContainers: []corev1.Container{
								{Name: "init", SecurityContext: defaultSC},
							},
						},
					},
				},
			},
			expectedContainerSC: overrideSC,
			expectedInitSC:      defaultSC,
		},
		{
			name: "SecurityContextOverrideDoesNotAffectInitContainers",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentMirror, Managed: true, Overrides: &v1.Override{SecurityContext: overrideSC}},
					},
				},
			},
			dep: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-quay-mirror",
					Labels:      map[string]string{"quay-component": "quay-mirror"},
					Annotations: map[string]string{"quay-component": "mirror"},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "quay-mirror", SecurityContext: defaultSC},
							},
							InitContainers: []corev1.Container{
								{Name: "quay-mirror-init", SecurityContext: defaultSC},
							},
						},
					},
				},
			},
			expectedContainerSC: overrideSC,
			expectedInitSC:      defaultSC,
		},
		{
			name: "NoOverrideRetainsDefaults",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentQuay, Managed: true},
					},
				},
			},
			dep: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-quay-app",
					Labels:      map[string]string{"quay-component": "quay-app"},
					Annotations: map[string]string{"quay-component": "quay"},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "quay-app", SecurityContext: defaultSC},
							},
						},
					},
				},
			},
			expectedContainerSC: defaultSC,
			expectedInitSC:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qctx := quaycontext.NewQuayRegistryContext()
			result, err := Process(tt.quay, qctx, tt.dep, false)
			assert.NoError(t, err)

			dep, ok := result.(*appsv1.Deployment)
			assert.True(t, ok)

			for _, c := range dep.Spec.Template.Spec.Containers {
				assert.Equal(t, tt.expectedContainerSC, c.SecurityContext)
			}
			for _, c := range dep.Spec.Template.Spec.InitContainers {
				if tt.expectedInitSC != nil {
					assert.Equal(t, tt.expectedInitSC, c.SecurityContext)
				}
			}
		})
	}
}

func TestProcessJobSecurityContextOverride(t *testing.T) {
	overrideSC := &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(false),
		AllowPrivilegeEscalation: boolPtr(true),
	}

	defaultSC := &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(true),
		AllowPrivilegeEscalation: boolPtr(false),
	}

	tests := []struct {
		name       string
		quay       *v1.QuayRegistry
		job        *batchv1k8s.Job
		expectedSC *corev1.SecurityContext
	}{
		{
			name: "JobSecurityContextOverrideFromQuayComponent",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentQuay, Managed: true, Overrides: &v1.Override{SecurityContext: overrideSC}},
					},
				},
			},
			job: &batchv1k8s.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-quay-app-upgrade",
					Labels: map[string]string{"quay-component": "quay-app-upgrade"},
				},
				Spec: batchv1k8s.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "quay-app-upgrade", SecurityContext: defaultSC},
							},
						},
					},
				},
			},
			expectedSC: overrideSC,
		},
		{
			name: "JobWithoutOverrideRetainsDefaults",
			quay: &v1.QuayRegistry{
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: v1.ComponentQuay, Managed: true},
					},
				},
			},
			job: &batchv1k8s.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-quay-app-upgrade",
					Labels: map[string]string{"quay-component": "quay-app-upgrade"},
				},
				Spec: batchv1k8s.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "quay-app-upgrade", SecurityContext: defaultSC},
							},
						},
					},
				},
			},
			expectedSC: defaultSC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qctx := quaycontext.NewQuayRegistryContext()
			result, err := Process(tt.quay, qctx, tt.job, false)
			assert.NoError(t, err)

			job, ok := result.(*batchv1k8s.Job)
			assert.True(t, ok)

			for _, c := range job.Spec.Template.Spec.Containers {
				assert.Equal(t, tt.expectedSC, c.SecurityContext)
			}
		})
	}
}

func TestApplyPostgresTLS(t *testing.T) {
	makePostgresDep := func(name, component string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: map[string]string{"quay-component": component},
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "postgres",
								Image: "quay.io/sclorg/postgresql-13-c9s:latest",
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "postgres-data",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "quay-postgres-13",
									},
								},
							},
						},
					},
				},
			},
		}
	}

	t.Run("injects init container and volume when TLS enabled", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
				},
			},
		}
		dep := makePostgresDep("test-quay-database", "postgres")

		applyPostgresTLS(quay, dep, v1.ComponentPostgres)

		assert.Len(t, dep.Spec.Template.Spec.InitContainers, 1)
		assert.Equal(t, "postgres-tls-init", dep.Spec.Template.Spec.InitContainers[0].Name)
		assert.Equal(t, "quay.io/sclorg/postgresql-13-c9s:latest", dep.Spec.Template.Spec.InitContainers[0].Image)

		// Check projected volume added with correct mode
		var found bool
		for _, vol := range dep.Spec.Template.Spec.Volumes {
			if vol.Name == "postgres-tls-certs" {
				found = true
				assert.NotNil(t, vol.Projected)
				assert.Equal(t, "test-postgres-tls", vol.Projected.Sources[0].Secret.Name)
				assert.NotNil(t, vol.Projected.DefaultMode)
				assert.Equal(t, int32(0600), *vol.Projected.DefaultMode)
			}
		}
		assert.True(t, found, "expected postgres-tls-certs volume")

		// Check init container only mounts the data volume (for patching postgresql.conf)
		initMounts := dep.Spec.Template.Spec.InitContainers[0].VolumeMounts
		assert.Len(t, initMounts, 1)
		assert.Equal(t, "postgres-data", initMounts[0].Name)
		assert.Equal(t, "/var/lib/pgsql/data", initMounts[0].MountPath)

		// Check main container has the TLS certs volume mount
		mainMounts := dep.Spec.Template.Spec.Containers[0].VolumeMounts
		var tlsMountFound bool
		for _, m := range mainMounts {
			if m.Name == "postgres-tls-certs" {
				tlsMountFound = true
				assert.Equal(t, "/tls-certs", m.MountPath)
				assert.True(t, m.ReadOnly)
			}
		}
		assert.True(t, tlsMountFound, "expected postgres-tls-certs mount on main container")
	})

	t.Run("cleanup init container when TLS not enabled", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true},
				},
			},
		}
		dep := makePostgresDep("test-quay-database", "postgres")

		applyPostgresTLS(quay, dep, v1.ComponentPostgres)

		assert.Len(t, dep.Spec.Template.Spec.InitContainers, 1)
		assert.Equal(t, "postgres-tls-init", dep.Spec.Template.Spec.InitContainers[0].Name)
		assert.Contains(t, dep.Spec.Template.Spec.InitContainers[0].Command[2], "sed")
		assert.Len(t, dep.Spec.Template.Spec.Volumes, 1) // only postgres-data, no TLS volume
	})

	t.Run("uses secretRef name when provided", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
						Enabled:   true,
						SecretRef: &corev1.LocalObjectReference{Name: "my-custom-certs"},
					}}},
				},
			},
		}
		dep := makePostgresDep("test-quay-database", "postgres")

		applyPostgresTLS(quay, dep, v1.ComponentPostgres)

		var secretName string
		for _, vol := range dep.Spec.Template.Spec.Volumes {
			if vol.Name == "postgres-tls-certs" && vol.Projected != nil {
				secretName = vol.Projected.Sources[0].Secret.Name
			}
		}
		assert.Equal(t, "my-custom-certs", secretName)
	})

	t.Run("works for clairpostgres component", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
				},
			},
		}
		dep := makePostgresDep("test-clair-postgres", "clairpostgres")

		applyPostgresTLS(quay, dep, v1.ComponentClairPostgres)

		assert.Len(t, dep.Spec.Template.Spec.InitContainers, 1)

		var secretName string
		for _, vol := range dep.Spec.Template.Spec.Volumes {
			if vol.Name == "postgres-tls-certs" && vol.Projected != nil {
				secretName = vol.Projected.Sources[0].Secret.Name
			}
		}
		assert.Equal(t, "test-clairpostgres-tls", secretName)
	})
}

func TestProcessDatabaseDeploymentWithTLS(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-quay-database",
			Labels:      map[string]string{"quay-component": "postgres"},
			Annotations: map[string]string{"quay-component": "postgres"},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"quay-registry-hostname":         "should-be-removed",
						"quay-buildmanager-hostname":     "should-be-removed",
						"quay-operator-service-endpoint": "should-be-removed",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "quay.io/sclorg/postgresql-13-c9s:latest",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "postgres-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "quay-postgres-13",
								},
							},
						},
					},
				},
			},
		},
	}

	qctx := quaycontext.NewQuayRegistryContext()
	result, err := Process(quay, qctx, dep, false)
	assert.NoError(t, err)

	processed := result.(*appsv1.Deployment)

	// TLS init container injected
	assert.Len(t, processed.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, "postgres-tls-init", processed.Spec.Template.Spec.InitContainers[0].Name)

	// Projected volume added
	var foundTLSVol bool
	for _, vol := range processed.Spec.Template.Spec.Volumes {
		if vol.Name == "postgres-tls-certs" {
			foundTLSVol = true
		}
	}
	assert.True(t, foundTLSVol, "expected postgres-tls-certs volume from Process()")

	// DB-specific annotation cleanup still happens
	_, hasHostname := processed.Spec.Template.Annotations["quay-registry-hostname"]
	_, hasBuildMgr := processed.Spec.Template.Annotations["quay-buildmanager-hostname"]
	_, hasEndpoint := processed.Spec.Template.Annotations["quay-operator-service-endpoint"]
	assert.False(t, hasHostname, "quay-registry-hostname should be removed")
	assert.False(t, hasBuildMgr, "quay-buildmanager-hostname should be removed")
	assert.False(t, hasEndpoint, "quay-operator-service-endpoint should be removed")

	// fieldGroupsAnnotation should NOT be present (DB deployments return early)
	_, hasFieldGroups := processed.Spec.Template.Annotations[fieldGroupsAnnotation]
	assert.False(t, hasFieldGroups, "database deployments should not have fieldGroupsAnnotation")
}

func TestApplyClairDBTLS(t *testing.T) {
	makeClairDep := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-clair-app",
				Labels:      map[string]string{"quay-component": "clair"},
				Annotations: map[string]string{"quay-component": "clair"},
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "clair-app"},
						},
					},
				},
			},
		}
	}

	t.Run("injects volume and mount when clairpostgres TLS enabled", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
					{Kind: "clair", Managed: true},
				},
			},
		}
		dep := makeClairDep()

		applyClairDBTLS(quay, dep)

		var foundVol bool
		for _, vol := range dep.Spec.Template.Spec.Volumes {
			if vol.Name == "clair-db-tls" {
				foundVol = true
				assert.NotNil(t, vol.Projected)
				assert.Equal(t, "test-clairpostgres-ca", vol.Projected.Sources[0].Secret.Name)
			}
		}
		assert.True(t, foundVol, "expected clair-db-tls volume")

		var foundMount bool
		for _, m := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
			if m.Name == "clair-db-tls" {
				foundMount = true
				assert.Equal(t, "/clair-db-tls", m.MountPath)
				assert.True(t, m.ReadOnly)
			}
		}
		assert.True(t, foundMount, "expected clair-db-tls volume mount")
	})

	t.Run("no changes when clairpostgres TLS not enabled", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "clairpostgres", Managed: true},
					{Kind: "clair", Managed: true},
				},
			},
		}
		dep := makeClairDep()

		applyClairDBTLS(quay, dep)

		assert.Len(t, dep.Spec.Template.Spec.Volumes, 0)
		assert.Len(t, dep.Spec.Template.Spec.Containers[0].VolumeMounts, 0)
	})

	t.Run("uses secretRef for CA when provided", func(t *testing.T) {
		quay := &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
						Enabled:   true,
						SecretRef: &corev1.LocalObjectReference{Name: "my-ca-secret"},
					}}},
					{Kind: "clair", Managed: true},
				},
			},
		}
		dep := makeClairDep()

		applyClairDBTLS(quay, dep)

		var secretName string
		for _, vol := range dep.Spec.Template.Spec.Volumes {
			if vol.Name == "clair-db-tls" && vol.Projected != nil {
				secretName = vol.Projected.Sources[0].Secret.Name
			}
		}
		assert.Equal(t, "my-ca-secret", secretName)
	})
}
