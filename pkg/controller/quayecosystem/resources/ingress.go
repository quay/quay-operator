package resources

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetQuayIngressDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem, hostname string, tlsSecretName string, isQuayConfig bool) *networkingv1beta1.Ingress {

	ingress := &networkingv1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: networkingv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
	}

	var servicePort int32

	if isQuayConfig {
		servicePort = 443
		ingress.ObjectMeta.Annotations = quayEcosystem.Spec.Quay.ExternalAccess.ConfigAnnotations
	} else {
		servicePort = GetQuayServicePort(*quayEcosystem)
		ingress.ObjectMeta.Annotations = quayEcosystem.Spec.Quay.ExternalAccess.Annotations
	}

	ingress.Spec = networkingv1beta1.IngressSpec{
		Rules: []networkingv1beta1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: networkingv1beta1.IngressRuleValue{
					HTTP: &networkingv1beta1.HTTPIngressRuleValue{
						Paths: []networkingv1beta1.HTTPIngressPath{
							{
								Path: "/",
								Backend: networkingv1beta1.IngressBackend{
									ServiceName: meta.Name,
									ServicePort: intstr.FromInt(int(servicePort)),
								},
							},
						},
					},
				},
			},
		},
	}

	if (isQuayConfig == true) || !quayEcosystem.IsInsecureQuay() {
		ingress.Spec.TLS = []networkingv1beta1.IngressTLS{
			networkingv1beta1.IngressTLS{
				SecretName: tlsSecretName,
			},
		}
	}

	return ingress

}
