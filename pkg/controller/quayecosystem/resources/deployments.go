package resources

import (
	"path/filepath"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
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

	envVars := []corev1.EnvVar{}

	if quayConfiguration.ValidProvidedRedisPasswordSecret {
		envVars = append(envVars, corev1.EnvVar{
			Name: constants.RedisPasswordEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: quayConfiguration.QuayEcosystem.Spec.Redis.CredentialsSecretName,
					},
					Key: constants.RedisPasswordKey,
				},
			},
		})
	}

	envVars = utils.MergeEnvVars(envVars, quayConfiguration.QuayEcosystem.Spec.Redis.EnvVars)

	redisDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Redis.Image,
			Env:   envVars,
			Name:  meta.Name,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 6379,
			}},
			ReadinessProbe: quayConfiguration.QuayEcosystem.Spec.Redis.ReadinessProbe,
			LivenessProbe:  quayConfiguration.QuayEcosystem.Spec.Redis.LivenessProbe,
			Resources:      quayConfiguration.QuayEcosystem.Spec.Redis.Resources,
		}},
		ServiceAccountName: constants.RedisServiceAccount,
		NodeSelector:       quayConfiguration.QuayEcosystem.Spec.Redis.NodeSelector,
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
			Strategy: appsv1.DeploymentStrategy{
				Type: quayConfiguration.QuayEcosystem.Spec.Redis.DeploymentStrategy,
			},
		},
	}

	return redisDeployment

}

func GetQuayConfigDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)
	BuildQuayConfigResourceLabels(meta.Labels)

	envVars := []corev1.EnvVar{
		{
			Name:  constants.EncryptedRobotTokenMigrationPhase,
			Value: string(quayConfiguration.QuayEcosystem.Spec.Quay.MigrationPhase),
		},
		{
			Name:  constants.QuayConfigReadOnlyEnvName,
			Value: constants.QuayConfigReadOnlyValues,
		},
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
	}

	envVars = utils.MergeEnvVars(envVars, quayConfiguration.QuayEcosystem.Spec.Quay.ConfigEnvVars)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Image,
			Name:  constants.QuayContainerConfigName,
			Env:   envVars,
			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.QuayHTTPContainerPort,
				Name:          "http",
			}, {
				ContainerPort: constants.QuayHTTPSContainerPort,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      constants.QuayConfigVolumeName,
				MountPath: constants.QuayConfigVolumePath,
				ReadOnly:  false,
			}},
			Resources: quayConfiguration.QuayEcosystem.Spec.Quay.ConfigResources,
			ReadinessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 10,
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: constants.QuayHTTPSContainerPort},
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 30,
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: constants.QuayHTTPSContainerPort},
					},
				},
			},
		}},
		NodeSelector:       quayConfiguration.QuayEcosystem.Spec.Quay.NodeSelector,
		ServiceAccountName: constants.QuayServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: constants.QuayConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: getBaselineQuayVolumeProjections(quayConfiguration),
				},
			}}}}

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
			Strategy: appsv1.DeploymentStrategy{
				Type: quayConfiguration.QuayEcosystem.Spec.Quay.DeploymentStrategy,
			},
		},
	}

	return quayDeployment
}

func GetQuayRepoMirrorDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetQuayRepoMirrorResourcesName(quayConfiguration.QuayEcosystem)
	BuildQuayRepoMirrorResourceLabels(meta.Labels)

	mirrorReplicas := utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Quay.MirrorReplicas, &constants.OneInt)

	envVars := []corev1.EnvVar{
		{
			Name:  constants.QuayEntryName,
			Value: constants.QuayEntryRepoMirrorValue,
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
	}

	envVars = utils.MergeEnvVars(envVars, quayConfiguration.QuayEcosystem.Spec.Quay.RepoMirrorEnvVars)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Image,
			Name:  constants.QuayContainerRepoMirrorName,
			Env:   envVars,
			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.QuayRepoMirrorContainerPort,
				Name:          "http",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      constants.QuayConfigVolumeName,
				MountPath: constants.QuayConfigVolumePath,
				ReadOnly:  false,
			}},
			Resources: quayConfiguration.QuayEcosystem.Spec.Quay.RepoMirrorResources,
			ReadinessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 10,
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: constants.QuayRepoMirrorContainerPort},
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold:    3,
				InitialDelaySeconds: 30,
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: constants.QuayRepoMirrorContainerPort},
					},
				},
			},
		}},
		NodeSelector:       quayConfiguration.QuayEcosystem.Spec.Quay.NodeSelector,
		ServiceAccountName: constants.QuayServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: constants.QuayConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: getBaselineQuayVolumeProjections(quayConfiguration),
				},
			}}}}

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
			Replicas: mirrorReplicas.(*int32),
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: quayDeploymentPodSpec,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: quayConfiguration.QuayEcosystem.Spec.Quay.DeploymentStrategy,
			},
		},
	}

	return quayDeployment
}

func GetQuayDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetQuayResourcesName(quayConfiguration.QuayEcosystem)
	BuildQuayResourceLabels(meta.Labels)

	configEnvVars := []corev1.EnvVar{
		{
			Name:  constants.EncryptedRobotTokenMigrationPhase,
			Value: string(quayConfiguration.QuayEcosystem.Spec.Quay.MigrationPhase),
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
	}

	configEnvVars = utils.MergeEnvVars(configEnvVars, quayConfiguration.QuayEcosystem.Spec.Quay.EnvVars)

	configVolumeSources := getBaselineQuayVolumeProjections(quayConfiguration)

	configVolume := corev1.Volume{
		Name: constants.QuayConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: configVolumeSources,
			},
		}}

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Quay.Image,
			Name:  constants.QuayContainerAppName,
			Env:   configEnvVars,
			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.QuayHTTPContainerPort,
				Name:          "http",
			}, {
				ContainerPort: constants.QuayHTTPSContainerPort,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      constants.QuayConfigVolumeName,
				MountPath: constants.QuayConfigVolumePath,
				ReadOnly:  false,
			}},
			ReadinessProbe: quayConfiguration.QuayEcosystem.Spec.Quay.ReadinessProbe,
			Resources:      quayConfiguration.QuayEcosystem.Spec.Quay.Resources,
			LivenessProbe:  quayConfiguration.QuayEcosystem.Spec.Quay.LivenessProbe,
		}},
		ServiceAccountName: constants.QuayServiceAccount,
		Volumes:            []corev1.Volume{configVolume},
		NodeSelector:       quayConfiguration.QuayEcosystem.Spec.Quay.NodeSelector,
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName) {
		quayDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Quay.ImagePullSecretName,
		},
		}
	}

	for _, registryBackend := range quayConfiguration.RegistryBackends {

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
			Strategy: appsv1.DeploymentStrategy{
				Type: quayConfiguration.QuayEcosystem.Spec.Quay.DeploymentStrategy,
			},
		},
	}

	return quayDeployment
}

func GetClairDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration) *appsv1.Deployment {

	meta.Name = GetClairResourcesName(quayConfiguration.QuayEcosystem)
	BuildClairResourceLabels(meta.Labels)

	envVars := []corev1.EnvVar{}

	envVars = utils.MergeEnvVars(envVars, quayConfiguration.QuayEcosystem.Spec.Clair.EnvVars)

	clairConfigVolumeProjections := []corev1.VolumeProjection{

		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetSecurityScannerSecretName(quayConfiguration.QuayEcosystem),
				},
				Items: []corev1.KeyToPath{
					corev1.KeyToPath{
						Key:  constants.SecurityScannerServiceSecretKey,
						Path: constants.SecurityScannerServiceSecretKey,
					},
				},
			},
		},
		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetClairSSLSecretName(quayConfiguration.QuayEcosystem),
				},
				Items: []corev1.KeyToPath{
					corev1.KeyToPath{
						Key:  corev1.TLSPrivateKeyKey,
						Path: constants.ClairSSLPrivateKeySecretKey,
					},
					corev1.KeyToPath{
						Key:  corev1.TLSCertKey,
						Path: constants.ClairSSLCertificateSecretKey,
					},
				},
			},
		},
		{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetClairConfigMapName(quayConfiguration.QuayEcosystem),
				},
				Items: []corev1.KeyToPath{
					corev1.KeyToPath{
						Key:  constants.ClairConfigFileKey,
						Path: constants.ClairConfigFileKey,
					},
				},
			},
		},
	}

	// Add configuration files
	if !utils.IsZeroOfUnderlyingType(quayConfiguration.ClairConfigFiles) {
		clairConfigVolumeProjections = append(clairConfigVolumeProjections, getConfigVolumeProjections(quayConfiguration.ClairConfigFiles, redhatcopv1alpha1.ConfigConfigFileType)...)
	}

	clairVolumes := []corev1.Volume{corev1.Volume{
		Name: "configvolume",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: clairConfigVolumeProjections,
			},
		},
	}}

	clairVolumeMounts := []corev1.VolumeMount{corev1.VolumeMount{
		Name:      "configvolume",
		MountPath: constants.ClairConfigVolumePath,
	}}

	// Configure Quay Certificate and Extra CA Certificates
	clairCertificateVolumeProjections := []corev1.VolumeProjection{}

	if !quayConfiguration.QuayEcosystem.IsInsecureQuay() {
		clairCertificateVolumeProjections = append(clairCertificateVolumeProjections, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetQuaySecretName(quayConfiguration.QuayEcosystem),
				},
				Items: []corev1.KeyToPath{
					corev1.KeyToPath{
						Key:  constants.QuayAppConfigSSLCertificateSecretKey,
						Path: constants.QuaySSLCertificate,
					},
				},
			},
		})
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.ClairConfigFiles) {
		clairCertificateVolumeProjections = append(clairCertificateVolumeProjections, getConfigVolumeProjections(quayConfiguration.ClairConfigFiles, redhatcopv1alpha1.ExtraCaCertConfigFileType)...)
	}

	clairVolumes = append(clairVolumes, corev1.Volume{
		Name: "sslvolume",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: clairCertificateVolumeProjections,
			},
		},
	})

	// Iterate over certificate projections
	for _, clairCertificateVolumeProjection := range clairCertificateVolumeProjections {

		// We only support secrets currently for ConfigFiles
		if !utils.IsZeroOfUnderlyingType(clairCertificateVolumeProjection.Secret) {
			for _, secretItems := range clairCertificateVolumeProjection.Secret.Items {
				clairVolumeMounts = append(clairVolumeMounts, corev1.VolumeMount{
					Name:      "sslvolume",
					MountPath: filepath.Join(constants.ClairTrustCaDir, secretItems.Path),
					SubPath:   secretItems.Path,
				})
			}

		}

	}

	clairDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: quayConfiguration.QuayEcosystem.Spec.Clair.Image,
			Name:  constants.ClairContainerName,
			Env:   envVars,
			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.ClairPort,
				Name:          "clair-api",
			}, {
				ContainerPort: constants.ClairHealthPort,
				Name:          "clair-health",
			}},
			Resources:      quayConfiguration.QuayEcosystem.Spec.Clair.Resources,
			VolumeMounts:   clairVolumeMounts,
			ReadinessProbe: quayConfiguration.QuayEcosystem.Spec.Clair.ReadinessProbe,
			LivenessProbe:  quayConfiguration.QuayEcosystem.Spec.Clair.LivenessProbe,
		}},
		NodeSelector:       quayConfiguration.QuayEcosystem.Spec.Clair.NodeSelector,
		ServiceAccountName: constants.ClairServiceAccount,
		Volumes:            clairVolumes,
	}

	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Clair.ImagePullSecretName) {
		clairDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: quayConfiguration.QuayEcosystem.Spec.Clair.ImagePullSecretName,
		},
		}
	}

	clairReplicas := utils.CheckValue(quayConfiguration.QuayEcosystem.Spec.Clair.Replicas, &constants.OneInt)

	clairDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: clairReplicas.(*int32),
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: clairDeploymentPodSpec,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: quayConfiguration.QuayEcosystem.Spec.Clair.DeploymentStrategy,
			},
		},
	}

	return clairDeployment
}

func GetDatabaseDeploymentDefinition(meta metav1.ObjectMeta, quayConfiguration *QuayConfiguration, database *redhatcopv1alpha1.Database, databaseComponent constants.DatabaseComponent) *appsv1.Deployment {

	envVars := []corev1.EnvVar{
		{
			Name: "POSTGRESQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: utils.CheckValue(database.CredentialsSecretName, meta.Name).(string),
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
						Name: utils.CheckValue(database.CredentialsSecretName, meta.Name).(string),
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
						Name: utils.CheckValue(database.CredentialsSecretName, meta.Name).(string),
					},
					Key: constants.DatabaseCredentialsDatabaseKey,
				},
			},
		},
	}

	envVars = utils.MergeEnvVars(envVars, database.EnvVars)

	databaseDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image:     database.Image,
			Name:      meta.Name,
			Env:       envVars,
			Resources: database.Resources,
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      constants.PostgresDataVolumeName,
				MountPath: constants.PostgresDataVolumePath,
			}},
			LivenessProbe:  database.LivenessProbe,
			ReadinessProbe: database.ReadinessProbe,

			Ports: []corev1.ContainerPort{{
				ContainerPort: constants.PostgreSQLPort,
			}},
		}},
		NodeSelector: database.NodeSelector,
		Volumes:      []corev1.Volume{},
	}

	if !utils.IsZeroOfUnderlyingType(database.ImagePullSecretName) {
		databaseDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: database.ImagePullSecretName,
		},
		}
	}

	if !utils.IsZeroOfUnderlyingType(database.VolumeSize) {

		databaseDeploymentPodSpec.Volumes = append(databaseDeploymentPodSpec.Volumes, corev1.Volume{
			Name: constants.PostgresDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: GetDatabaseResourceName(quayConfiguration.QuayEcosystem, databaseComponent),
				},
			},
		})

	} else {
		databaseDeploymentPodSpec.Volumes = append(databaseDeploymentPodSpec.Volumes, corev1.Volume{
			Name: constants.PostgresDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	if !utils.IsZeroOfUnderlyingType(database.Memory) || !utils.IsZeroOfUnderlyingType(database.CPU) {
		databaseResourceRequirements := corev1.ResourceRequirements{}
		databaseResourceLimits := corev1.ResourceList{}
		databaseResourceRequests := corev1.ResourceList{}

		if !utils.IsZeroOfUnderlyingType(database.Memory) {
			databaseResourceLimits[corev1.ResourceMemory] = resource.MustParse(database.Memory)
			databaseResourceRequests[corev1.ResourceMemory] = resource.MustParse(database.Memory)
		}

		if !utils.IsZeroOfUnderlyingType(database.CPU) {
			databaseResourceLimits[corev1.ResourceCPU] = resource.MustParse(database.CPU)
			databaseResourceRequests[corev1.ResourceCPU] = resource.MustParse(database.CPU)
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
			Replicas: database.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: meta.Labels,
				},
				Spec: databaseDeploymentPodSpec,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: database.DeploymentStrategy,
			},
		},
	}

	return databaseDeployment

}

func getBaselineQuayVolumeProjections(quayConfiguration *QuayConfiguration) []corev1.VolumeProjection {

	configVolumeProjections := []corev1.VolumeProjection{
		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetQuaySecretName(quayConfiguration.QuayEcosystem),
				},
			},
		},
	}

	// Add User Defined Config Files
	if !utils.IsZeroOfUnderlyingType(quayConfiguration.QuayConfigFiles) {

		configVolumeProjections = append(configVolumeProjections, getConfigVolumeProjections(quayConfiguration.QuayConfigFiles, redhatcopv1alpha1.ConfigConfigFileType)...)
	}

	return configVolumeProjections

}

func getConfigVolumeProjections(inputConfigFiles []redhatcopv1alpha1.ConfigFiles, configFileType redhatcopv1alpha1.ConfigFileType) []corev1.VolumeProjection {

	configVolumeSources := []corev1.VolumeProjection{}

	for _, configFiles := range inputConfigFiles {

		configFilesKeyToPaths := []corev1.KeyToPath{}

		if !utils.IsZeroOfUnderlyingType(configFiles.Files) {

			for _, configFile := range configFiles.Files {

				if utils.IsZeroOfUnderlyingType(configFile.Type) && redhatcopv1alpha1.ExtraCaCertConfigFileType == configFileType {
					continue
				} else {
					if configFile.Type != configFileType {
						continue
					}
				}

				filename := ""

				if utils.IsZeroOfUnderlyingType(configFile.Filename) {
					filename = configFile.Key
				} else {
					filename = configFile.Filename
				}

				configFilesKeyToPaths = append(configFilesKeyToPaths, corev1.KeyToPath{
					Key:  configFile.Key,
					Path: filename,
				})

			}
		}

		if len(configFilesKeyToPaths) > 0 {
			configVolumeSources = append(configVolumeSources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configFiles.SecretName,
					},
					Items: configFilesKeyToPaths,
				},
			})
		}

	}

	return configVolumeSources
}
