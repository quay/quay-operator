package databases

import (
	"encoding/base64"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateDatabaseConfig(meta metav1.ObjectMeta, inputDatabase redhatcopv1alpha1.Database, credentials *corev1.Secret, defaultCredentials map[string]string) DatabaseConfig {
	database := DatabaseConfig{}
	database.Name = meta.Name
	database.Image = inputDatabase.Image
	database.LimitsMemory = inputDatabase.Memory
	database.RequestsMemory = inputDatabase.Memory
	database.LimitsCPU = inputDatabase.CPU
	database.RequestsCPU = inputDatabase.Memory
	database.VolumeSize = inputDatabase.VolumeSize
	database.Database = GetCredentialValue(constants.DatabaseCredentialsDatabaseKey, credentials, defaultCredentials)
	database.Username = GetCredentialValue(constants.DatabaseCredentialsUsernameKey, credentials, defaultCredentials)
	database.Password = GetCredentialValue(constants.DatabaseCredentialsPasswordKey, credentials, defaultCredentials)
	database.RootPassword = GetCredentialValue(constants.DatabaseCredentialsRootPasswordKey, credentials, defaultCredentials)
	database.CredentialsName = credentials.Name
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

func GetCredentialValue(key string, credentials *corev1.Secret, defaultCredentials map[string]string) string {

	if len(credentials.Data) != 0 {
		if val, ok := credentials.Data[key]; ok {
			return base64.StdEncoding.EncodeToString(val)
		}
	}

	return defaultCredentials[key]
}
