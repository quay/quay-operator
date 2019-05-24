package databases

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MySQLDatabase represents a PostgreSQL database
type MySQLDatabase struct{}

func (m *MySQLDatabase) GenerateResources(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem, database DatabaseConfig) ([]metav1.Object, error) {

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

func generateMySQLDatabaseResource(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem, database DatabaseConfig) (*appsv1.Deployment, error) {

	databaseDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: database.Image,
			Name:  meta.Name,
			Env: []corev1.EnvVar{
				{
					Name: "MYSQL_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: database.CredentialsName,
							},
							Key: constants.DatabaseCredentialsUsernameKey,
						},
					},
				},
				{
					Name: "MYSQL_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: database.CredentialsName,
							},
							Key: constants.DatabaseCredentialsPasswordKey,
						},
					},
				},
				{
					Name: "MYSQL_ROOT_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: database.CredentialsName,
							},
							Key: constants.DatabaseCredentialsRootPasswordKey,
						},
					},
				},
				{
					Name: "MYSQL_DATABASE",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: database.CredentialsName,
							},
							Key: constants.DatabaseCredentialsDatabaseKey,
						},
					},
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

func (m *MySQLDatabase) GetDefaultSecret(meta metav1.ObjectMeta, credentials map[string]string) *corev1.Secret {

	defaultSecret := &corev1.Secret{
		ObjectMeta: meta,
		StringData: map[string]string{
			constants.DatabaseCredentialsDatabaseKey:     credentials[constants.DatabaseCredentialsDatabaseKey],
			constants.DatabaseCredentialsUsernameKey:     credentials[constants.DatabaseCredentialsUsernameKey],
			constants.DatabaseCredentialsPasswordKey:     credentials[constants.DatabaseCredentialsPasswordKey],
			constants.DatabaseCredentialsRootPasswordKey: credentials[constants.DatabaseCredentialsRootPasswordKey],
		},
	}

	return defaultSecret

}

func (m *MySQLDatabase) ValidateProvidedSecret(secret *corev1.Secret) bool {

	for _, item := range []string{constants.DatabaseCredentialsDatabaseKey, constants.DatabaseCredentialsPasswordKey, constants.DatabaseCredentialsRootPasswordKey, constants.DatabaseCredentialsUsernameKey} {
		if _, found := secret.Data[item]; !found {
			return false
		}
	}
	return true

}
