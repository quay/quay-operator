package resources

import (
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetQuayPVCRegistryStorageDefinition(meta metav1.ObjectMeta, accessModes []corev1.PersistentVolumeAccessMode, pvSize string, storageClass *string) *corev1.PersistentVolumeClaim {

	registryStoragePVC := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(pvSize),
				},
			},
		},
	}

	if !utils.IsZeroOfUnderlyingType(storageClass) && len(*storageClass) != 0 {
		registryStoragePVC.Spec.StorageClassName = storageClass
	}

	return registryStoragePVC

}

func GetDatabasePVCDefinition(meta metav1.ObjectMeta, volumeSize string, storageClass *string) *corev1.PersistentVolumeClaim {

	databasePVCDefinition := &corev1.PersistentVolumeClaim{
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
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(volumeSize),
				},
			},
		},
	}

	if !utils.IsZeroOfUnderlyingType(storageClass) && len(*storageClass) != 0 {
		databasePVCDefinition.Spec.StorageClassName = storageClass
	}

	return databasePVCDefinition
}
