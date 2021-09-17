package middleware

import (
	"testing"

	route "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

const (
	tlsCert = "my-own-cert"
	tlsKey  = "my-own-key"
)

var processTests = []struct {
	name          string
	ctx           *quaycontext.QuayRegistryContext
	quay          *v1.QuayRegistry
	obj           client.Object
	expected      client.Object
	expectedError error
}{
	{
		"quayConfigBundle",
		&quaycontext.QuayRegistryContext{},
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
		&quaycontext.QuayRegistryContext{
			TLSCert: []byte(tlsCert),
			TLSKey:  []byte(tlsKey),
		},
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
		&quaycontext.QuayRegistryContext{
			TLSCert: []byte(tlsCert),
			TLSKey:  []byte(tlsKey),
		},
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
		&quaycontext.QuayRegistryContext{
			TLSCert: []byte(tlsCert),
			TLSKey:  []byte(tlsKey),
		},
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
}

func TestProcess(t *testing.T) {
	assert := assert.New(t)

	for _, test := range processTests {
		processedObj, err := Process(test.ctx, test.quay, test.obj)

		if test.expectedError != nil {
			assert.Error(err, test.name)
		} else {
			assert.Nil(err, test.name)
		}

		assert.Equal(test.expected, processedObj, test.name)
	}
}
