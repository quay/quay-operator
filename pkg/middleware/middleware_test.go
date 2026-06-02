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

func TestSTSVolumeInjection(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: v1.ComponentQuay, Managed: true},
			},
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-quay-app",
			Annotations: map[string]string{"quay-component": "quay-app"},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "quay-app"},
					},
				},
			},
		},
	}

	t.Run("STS disabled leaves deployment unchanged", func(t *testing.T) {
		qctx := quaycontext.NewQuayRegistryContext()
		result, err := Process(quay, qctx, dep.DeepCopy(), false)
		assert.NoError(t, err)

		d := result.(*appsv1.Deployment)
		for _, c := range d.Spec.Template.Spec.Containers {
			for _, env := range c.Env {
				assert.NotEqual(t, "AWS_SHARED_CREDENTIALS_FILE", env.Name)
			}
		}
		for _, vol := range d.Spec.Template.Spec.Volumes {
			assert.NotEqual(t, "sts-credentials", vol.Name)
		}
	})

	t.Run("STS enabled and provisioned injects volume and env", func(t *testing.T) {
		qctx := quaycontext.NewQuayRegistryContext()
		qctx.STSEnabled = true
		qctx.STSCredentialProvisioned = true
		qctx.STSCredentialSecretName = "test-quay-app-aws"

		result, err := Process(quay, qctx, dep.DeepCopy(), false)
		assert.NoError(t, err)

		d := result.(*appsv1.Deployment)

		foundVol := false
		for _, vol := range d.Spec.Template.Spec.Volumes {
			if vol.Name == "sts-credentials" {
				foundVol = true
				assert.Equal(t, "test-quay-app-aws", vol.Secret.SecretName)
			}
		}
		assert.True(t, foundVol, "expected sts-credentials volume")

		for _, c := range d.Spec.Template.Spec.Containers {
			foundMount := false
			for _, vm := range c.VolumeMounts {
				if vm.Name == "sts-credentials" {
					foundMount = true
					assert.Equal(t, "/var/run/secrets/cloud", vm.MountPath)
					assert.True(t, vm.ReadOnly)
				}
			}
			assert.True(t, foundMount, "expected sts-credentials volume mount")

			foundEnv := false
			for _, env := range c.Env {
				if env.Name == "AWS_SHARED_CREDENTIALS_FILE" {
					foundEnv = true
					assert.Equal(t, "/var/run/secrets/cloud/credentials", env.Value)
				}
			}
			assert.True(t, foundEnv, "expected AWS_SHARED_CREDENTIALS_FILE env var")
		}
	})
}
