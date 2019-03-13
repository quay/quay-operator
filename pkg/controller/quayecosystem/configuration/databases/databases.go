package databases

import (
	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateDatabaseConfig(meta metav1.ObjectMeta, inputDatabase copv1alpha1.Database) DatabaseConfig {
	database := DatabaseConfig{}
	database.Name = meta.Name
	database.Image = inputDatabase.Image
	database.LimitsMemory = inputDatabase.Memory
	database.RequestsMemory = inputDatabase.Memory
	database.LimitsCPU = inputDatabase.CPU
	database.RequestsCPU = inputDatabase.Memory
	database.VolumeSize = inputDatabase.VolumeSize
	database.Database = inputDatabase.DatabaseName
	database.Username = "quay"
	database.Password = "quayPassword"
	database.RootPassword = "rootPassword"
	return database
}

func GenerateDatabasePVC(meta metav1.ObjectMeta, database DatabaseConfig) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(database.VolumeSize),
				},
			},
		},
	}

}

func GenerateDatabaseServiceResource(meta metav1.ObjectMeta, port int) *corev1.Service {

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
