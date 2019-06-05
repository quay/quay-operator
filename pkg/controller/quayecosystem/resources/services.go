package resources

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetRedisServiceDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *corev1.Service {

	meta.Name = GetRedisResourcesName(quayEcosystem)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector: meta.Labels,
			Ports: []corev1.ServicePort{
				{
					Port:       6379,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(6379),
				},
			},
		},
	}

	service.ObjectMeta.Labels = BuildRedisResourceLabels(meta.Labels)

	return service
}

func GetQuayServiceDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *corev1.Service {

	meta.Name = GetQuayResourcesName(quayEcosystem)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector: meta.Labels,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	service.ObjectMeta.Labels = BuildQuayResourceLabels(meta.Labels)

	if quayEcosystem.Spec.Quay.EnableNodePortService {
		service.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}

	return service

}

func GetQuayConfigServiceDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *corev1.Service {

	meta.Name = GetQuayConfigResourcesName(quayEcosystem)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector: meta.Labels,
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(8443),
				},
			},
		},
	}

	service.ObjectMeta.Labels = BuildQuayConfigResourceLabels(meta.Labels)

	if quayEcosystem.Spec.Quay.EnableNodePortService {
		service.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}

	return service

}

func GetDatabaseServiceResourceDefinition(meta metav1.ObjectMeta, port int) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector: meta.Labels,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(port),
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(port),
				},
			},
		},
	}

	return service
}
