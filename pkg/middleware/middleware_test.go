package middleware

import (
	"fmt"
	"testing"

	route "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
)

const (
	tlsCert = "my-own-cert"
	tlsKey  = "my-own-key"
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
			processedObj, err := Process(test.quay, test.obj, false)
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
