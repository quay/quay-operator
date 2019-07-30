package resources

import (
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetRedisDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetRedisResourcesName(quayConfiguration.QuayEcosystem)

	redisDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Redis.Image,
			Name:  meta.Name,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 6379,
			}},
		}},
		ServiceAccountName: constants.RedisServiceAccount,
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.ImagePullSecretName) {
		redisDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Redis.ImagePullSecretName,
		},
		}
	}

	redisReplicas := utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Redis.Replicas, &constants.RedisReplicas)

	redisDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: redisReplicas.(*int32),
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: redisDeploymentPodSpec,
			},
		},
	}

	return redisDeployment

}

func GetQuayConfigDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)
	BuildQuayConfigResourceLabels(meta.Labels)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Image,
			Name:  constants.QuayContainerConfigName,
			Env: []corev1.EnvVar{
				{
					Name:  constants.QuayEntryName,
					Value: constants.QuayEntryConfigValue,
				},
				{
					Name: constants.QuayConfigPasswordName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: quayConfiguration.QuayConfigPasswordSecret,
							},
							Key: constants.QuayConfigPasswordKey,
						},
					},
				},
				{
					Name: constants.QuayNamespaceEnvironmentVariable,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
				Name:          "http",
			}, {
				ContainerPort: 8443,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      "configvolume",
				MountPath: "/conf/stack",
				ReadOnly:  false,
			}},
			ReadinessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 10,
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: 8443},
					},
				},
			},
		}},
		ServiceAccountName: constants.QuayServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: "configvolume",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: GetConfigMapSecretName(quayConfiguration.QuayEcosystem),
								},
							},
						},
					},
				},
			}}},
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName) {
		quayDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName,
		},
		}
	}

	quayDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: quayDeploymentPodSpec,
			},
		},
	}

	return quayDeployment
}

func GetQuayDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetQuayResourcesName(quayConfiguration.QuayEcosystem)
	BuildQuayResourceLabels(meta.Labels)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Image,
			Name:  constants.QuayContainerAppName,
			Env: []corev1.EnvVar{
				{
					Name: constants.QuayNamespaceEnvironmentVariable,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
				Name:          "http",
			}, {
				ContainerPort: 8443,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      "configvolume",
				MountPath: "/conf/stack",
				ReadOnly:  false,
			}},
			ReadinessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 10,
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/health/instance",
						Port:   intstr.IntOrString{IntVal: 8443},
						Scheme: "HTTPS",
					},
				},
			},
		}},
		ServiceAccountName: constants.QuayServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: "configvolume",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: GetConfigMapSecretName(quayConfiguration.QuayEcosystem),
								},
							},
						},
					},
				},
			},
		}},
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName) {
		quayDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName,
		},
		}
	}

	for _, registryBackend := range quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends {

		if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Local) {

			quayDeploymentPodSpec.Containers[0].VolumeMounts = append(quayDeploymentPodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      GetRegistryStorageVolumeName(quayConfiguration.QuayEcosystem, registryBackend.Name),
				MountPath: registryBackend.RegistryBackendSource.Local.StoragePath,
				ReadOnly:  false,
			})

			if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) {
				quayDeploymentPodSpec.Volumes = append(quayDeploymentPodSpec.Volumes, corev1.Volume{
					Name: GetRegistryStorageVolumeName(quayConfiguration.QuayEcosystem, registryBackend.Name),
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: GetRegistryStorageVolumeName(quayConfiguration.QuayEcosystem, registryBackend.Name),
						},
					},
				})

			} else {
				quayDeploymentPodSpec.Volumes = append(quayDeploymentPodSpec.Volumes, corev1.Volume{
					Name: GetRegistryStorageVolumeName(quayConfiguration.QuayEcosystem, registryBackend.Name),
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})

			}

		}

	}

	quayReplicas := utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Quay.Replicas, &constants.OneInt)

	quayDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: quayReplicas.(*int32),
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: quayDeploymentPodSpec,
			},
		},
	}

	return quayDeployment
}

func GetDatabaseDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	databaseDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image,
			Name:  meta.Name,
			Env: []corev1.EnvVar{
				{
					Name: "POSTGRESQL_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CredentialsSecretName, GetQuayDatabaseName(quayConfiguration.QuayEcosystem)).(string),
							},
							Key: constants.DatabaseCredentialsUsernameKey,
						},
					},
				},
				{
					Name: "POSTGRESQL_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CredentialsSecretName, GetQuayDatabaseName(quayConfiguration.QuayEcosystem)).(string),
							},
							Key: constants.DatabaseCredentialsPasswordKey,
						},
					},
				},
				{
					Name: "POSTGRESQL_DATABASE",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CredentialsSecretName, GetQuayDatabaseName(quayConfiguration.QuayEcosystem)).(string),
							},
							Key: constants.DatabaseCredentialsDatabaseKey,
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(constants.PostgreSQLPort),
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      1,
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"/usr/libexec/check-container", "--live"},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      1,
			},

			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.PostgreSQLPort,
			}},
		}},
		Volumes: []corev1.Volume{},
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.ImagePullSecretName) {
		databaseDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Quay.Database.ImagePullSecretName,
		},
		}
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize) {
		databaseDeploymentPodSpec.Containers[0].VolumeMounts = append(databaseDeploymentPodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: "/var/lib/pgsql/data",
		})

		databaseDeploymentPodSpec.Volumes = append(databaseDeploymentPodSpec.Volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: GetQuayDatabaseName(quayConfiguration.QuayEcosystem),
				},
			},
		})

	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Memory) || !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CPU) {
		databaseResourceRequirements := corev1.ResourceRequirements{}
		databaseResourceLimits := corev1.ResourceList{}
		databaseResourceRequests := corev1.ResourceList{}

		if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Memory) {
			databaseResourceLimits[corev1.ResourceMemory] = resource.MustParse(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Memory)
			databaseResourceRequests[corev1.ResourceMemory] = resource.MustParse(quayConfiguration.QuayEcosystem.Spec.Quay.Database.Memory)
		}

		if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CPU) {
			databaseResourceLimits[corev1.ResourceCPU] = resource.MustParse(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CPU)
			databaseResourceRequests[corev1.ResourceCPU] = resource.MustParse(quayConfiguration.QuayEcosystem.Spec.Quay.Database.CPU)
		}

		databaseResourceRequirements.Requests = databaseResourceRequests
		databaseResourceRequirements.Limits = databaseResourceLimits

		databaseDeploymentPodSpec.Containers[0].Resources = databaseResourceRequirements
	}
	databaseDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: quayConfiguration.QuayEcosystem.Spec.Quay.Database.Replicas,
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

	return databaseDeployment

}
