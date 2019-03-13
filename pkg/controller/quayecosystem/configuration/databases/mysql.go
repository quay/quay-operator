package databases

import (
	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MySQLDatabase represents a PostgreSQL database
type MySQLDatabase struct{}

func (m *MySQLDatabase) GenerateResources(meta metav1.ObjectMeta, quayEcosystem *copv1alpha1.QuayEcosystem, database DatabaseConfig) ([]metav1.Object, error) {

	var resources []metav1.Object

	service := GenerateDatabaseServiceResource(meta, constants.MySQLPort)
	resources = append(resources, service)

	deployment, err := generateMySQLDatabaseResource(meta, quayEcosystem, database)

	if err != nil {
		return nil, err
	}

	resources = append(resources, deployment)

	return resources, nil

}

func generateMySQLDatabaseResource(meta metav1.ObjectMeta, quayEcosystem *copv1alpha1.QuayEcosystem, database DatabaseConfig) (*appsv1.Deployment, error) {

	meta.Name = resources.GetQuayDatabaseName(quayEcosystem)

	databaseDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: database.Image,
			Name:  meta.Name,
			Env: []corev1.EnvVar{
				{
					Name:  "MYSQL_USER",
					Value: database.Username,
				},
				{
					Name:  "MYSQL_PASSWORD",
					Value: database.Password,
				},
				{
					Name:  "MYSQL_ROOT_PASSWORD",
					Value: database.RootPassword,
				},
				{
					Name:  "MYSQL_DATABASE",
					Value: database.Database,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/var/lib/mysql/data",
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(database.LimitsMemory),
				},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(constants.MySQLPort),
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      1,
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/sh", "-i", "-c", "MYSQL_PWD=\"$MYSQL_PASSWORD\" mysql -h 127.0.0.1 -u $MYSQL_USER -D $MYSQL_DATABASE -e 'SELECT 1'"},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      1,
			},

			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.MySQLPort,
			}},
		}},
		Volumes: []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: database.Name,
					},
				},
			},
		},
	}

	if len(quayEcosystem.Spec.ImagePullSecretName) != 0 {
		databaseDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayEcosystem.Spec.ImagePullSecretName,
		},
		}
	}

	databaseDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: quayEcosystem.Spec.Quay.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: databaseDeploymentPodSpec,
			},
		},
	}

	return databaseDeployment, nil

}
