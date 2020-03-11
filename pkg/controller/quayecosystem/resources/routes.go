package resources

import (
	routev1 "github.com/openshift/api/route/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetQuayConfigRouteDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *routev1.Route {

	meta.Name = GetQuayConfigResourcesName(quayEcosystem)

	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: meta.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(8443),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}

	route.ObjectMeta.Labels = BuildQuayConfigResourceLabels(meta.Labels)

	if !utils.IsZeroOfUnderlyingType(quayEcosystem.Spec.Quay.ExternalAccess.ConfigHostname) {
		route.Spec.Host = quayEcosystem.Spec.Quay.ExternalAccess.ConfigHostname
	}

	return route
}

func GetQuayRouteDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *routev1.Route {

	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: meta.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(int(quayEcosystem.GetQuayPort())),
			},
		},
	}

	if redhatcopv1alpha1.NoneTLSTerminationType != quayEcosystem.Spec.Quay.ExternalAccess.TLS.Termination {

		var termination routev1.TLSTerminationType
		switch quayEcosystem.Spec.Quay.ExternalAccess.TLS.Termination {
		case redhatcopv1alpha1.EdgeTLSTerminationType:
			termination = routev1.TLSTerminationEdge
		case redhatcopv1alpha1.PassthroughTLSTerminationType:
			termination = routev1.TLSTerminationPassthrough
		case redhatcopv1alpha1.ReencryptTLSTerminationType:
			termination = routev1.TLSTerminationReencrypt

		}

		route.Spec.TLS = &routev1.TLSConfig{
			Termination:                   termination,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}

	}

	route.ObjectMeta.Labels = BuildQuayResourceLabels(meta.Labels)

	if !utils.IsZeroOfUnderlyingType(quayEcosystem.Spec.Quay.ExternalAccess.Hostname) {
		route.Spec.Host = quayEcosystem.Spec.Quay.ExternalAccess.Hostname
	}

	return route

}
