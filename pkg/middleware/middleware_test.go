package middleware

import (
	"fmt"
	"testing"

	route "github.com/openshift/api/route/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func parseResourceString(s string) *resource.Quantity {
	resourceSize := resource.MustParse(s)
	return &resourceSize
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
	// Helper for getting string pointers
	strPtr := func(s string) *string { return &s }

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
			storageClassName:     strPtr("my-fast-storage"),
			expectedStorageClass: strPtr("my-fast-storage"),
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
			storageClassName:     strPtr("clair-storage"),
			expectedStorageClass: strPtr("clair-storage"),
		},
		{
			name:                   "Postgres with initial StorageClassName and no override",
			componentKind:          v1.ComponentPostgres,
			componentLabel:         "postgres",
			storageClassName:       nil,
			initialPVCStorageClass: strPtr("default-storage"),
			expectedStorageClass:   strPtr("default-storage"),
		},
		{
			name:                   "Postgres with initial StorageClassName and different override",
			componentKind:          v1.ComponentPostgres,
			componentLabel:         "postgres",
			storageClassName:       strPtr("override-storage"),
			initialPVCStorageClass: strPtr("initial-storage"),
			expectedStorageClass:   strPtr("override-storage"),
		},
		{
			name:                 "Irrelevant component (redis) with override, postgres PVC without",
			componentKind:        v1.ComponentRedis,       // Override set for Redis
			componentLabel:       "postgres",              // PVC is for Postgres
			storageClassName:     strPtr("redis-storage"), // This should not apply to the postgres PVC
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
