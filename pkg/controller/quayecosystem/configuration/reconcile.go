package configuration

import (
	"context"
	"fmt"

	"reflect"

	"github.com/sirupsen/logrus"

	ossecurityv1 "github.com/openshift/api/security/v1"

	routev1 "github.com/openshift/api/route/v1"
	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/databases"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/resources"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileQuayEcosystemConfiguration defines values required for Quay configuration
type ReconcileQuayEcosystemConfiguration struct {
	client        client.Client
	scheme        *runtime.Scheme
	quayEcosystem *copv1alpha1.QuayEcosystem
}

// New creates the structure for the Quay configuration
func New(client client.Client, scheme *runtime.Scheme,
	quayEcosystem *copv1alpha1.QuayEcosystem) *ReconcileQuayEcosystemConfiguration {
	return &ReconcileQuayEcosystemConfiguration{
		client:        client,
		scheme:        scheme,
		quayEcosystem: quayEcosystem,
	}
}

// Reconcile takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) Reconcile() (*reconcile.Result, error) {
	metaObject := resources.NewResourceObjectMeta(r.quayEcosystem)

	if err := r.createQuayConfigSecret(metaObject); err != nil {
		return &reconcile.Result{}, err
	}

	if err := r.createRBAC(metaObject); err != nil {
		logrus.Errorf("Failed to create RBAC: %v", err)
		return nil, err
	}

	if err := r.createQuayEcosystemServiceAccount(metaObject); err != nil {
		logrus.Errorf("Failed to create Service Account: %v", err)
		return nil, err
	}

	if err := r.configureSCC(metaObject); err != nil {
		logrus.Errorf("Failed to configure SCC: %v", err)
		return nil, err
	}

	// Redis
	if !r.quayEcosystem.Spec.Redis.Skip {
		if err := r.createRedisService(metaObject); err != nil {
			logrus.Errorf("Failed to create Redis service: %v", err)
			return nil, err
		}

		if err := r.redisDeployment(metaObject); err != nil {
			logrus.Errorf("Failed to create Redis deployment: %v", err)
			return nil, err
		}

	}

	// Database (PostgreSQL/MySQL)
	if !reflect.DeepEqual(copv1alpha1.Database{}, r.quayEcosystem.Spec.Quay.Database) {

		if err := r.createQuayDatabase(metaObject); err != nil {
			logrus.Errorf("Failed to create Quay database: %v", err)
			return nil, err
		}

	}

	// Quay Resources
	if err := r.createQuayService(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay service: %v", err)
		return nil, err
	}

	if err := r.createQuayConfigService(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay Config service: %v", err)
		return nil, err
	}

	if err := r.createQuayRoute(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay route: %v", err)
		return nil, err
	}

	if err := r.createQuayConfigRoute(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay Config route: %v", err)
		return nil, err
	}

	if !reflect.DeepEqual(copv1alpha1.RegistryStorage{}, r.quayEcosystem.Spec.Quay.RegistryStorage) {

		if err := r.quayRegistryStorage(metaObject); err != nil {
			logrus.Errorf("Failed to create registry storage: %v", err)
			return nil, err
		}

	}

	if err := r.quayDeployment(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay deployment: %v", err)
		return nil, err
	}

	if err := r.quayConfigDeployment(metaObject); err != nil {
		logrus.Errorf("Failed to create Quay Config deployment: %v", err)
		return nil, err
	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayDatabase(meta metav1.ObjectMeta) error {

	var database databases.Database

	// Update Metadata
	meta = resources.UpdateMetaWithName(meta, resources.GetQuayDatabaseName(r.quayEcosystem))
	resources.BuildQuayDatabaseResourceLabels(meta.Labels)

	switch r.quayEcosystem.Spec.Quay.Database.Type {
	case copv1alpha1.DatabaseMySQL:
		database = new(databases.MySQLDatabase)
	case copv1alpha1.DatabasePostgresql:
		database = new(databases.PostgreSQLDatabase)
	default:
		logrus.Warn("Unknown database type")
	}

	var existingValidSecret = false
	databaseSecret := &corev1.Secret{}

	// Check if Secret Exists
	if len(r.quayEcosystem.Spec.Quay.Database.CredentialsSecretName) != 0 {

		err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.quayEcosystem.Spec.Quay.Database.CredentialsSecretName, Namespace: r.quayEcosystem.ObjectMeta.Namespace}, databaseSecret)

		if err == nil && database.ValidateProvidedSecret(databaseSecret) {
			existingValidSecret = true
		}
	}

	// Create Secret if no valid secret found
	if !existingValidSecret {
		databaseSecret = database.GetDefaultSecret(meta, constants.DefaultQuayDatabaseCredentials)
		err := r.createResource(databaseSecret, r.quayEcosystem)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}

	}

	// Generate Database Object
	databaseConfig := databases.GenerateDatabaseConfig(meta, r.quayEcosystem.Spec.Quay.Database, databaseSecret, constants.DefaultQuayDatabaseCredentials)

	// Create PVC
	databasePvc := databases.GenerateDatabasePVC(meta, databaseConfig)

	err := r.createResource(databasePvc, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	resources, err := database.GenerateResources(meta, r.quayEcosystem, databaseConfig)

	if err != nil {
		return err
	}

	for _, resource := range resources {
		err = r.createResource(resource, r.quayEcosystem)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logrus.Errorf("Error applying Resource: %v", err)
			return err
		}
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetConfigMapSecretName(r.quayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		configSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}

		return r.createResource(configSecret, r.quayEcosystem)
	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayEcosystemServiceAccount(meta metav1.ObjectMeta) error {

	meta.Name = constants.QuayEcosystemServiceAccount

	found := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: constants.QuayEcosystemServiceAccount, Namespace: r.quayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		configServiceAccount := &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceAccount",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}

		return r.createResource(configServiceAccount, r.quayEcosystem)
	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRBAC(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetGenericResourcesName(r.quayEcosystem)

	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "put", "patch", "update", "create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"extensions", "apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "patch", "update", "watch"},
			},
		},
	}

	err := r.createOrUpdateResource(role)
	if err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     meta.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.QuayEcosystemServiceAccount,
				Namespace: meta.Namespace,
			},
		},
	}

	err = r.createOrUpdateResource(roleBinding)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayService(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.quayEcosystem)

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

	service.ObjectMeta.Labels = resources.BuildQuayResourceLabels(meta.Labels)

	if r.quayEcosystem.Spec.Quay.EnableNodePortService {
		service.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}

	err := r.createResource(service, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigService(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayConfigResourcesName(r.quayEcosystem)

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

	service.ObjectMeta.Labels = resources.BuildQuayConfigResourceLabels(meta.Labels)

	if r.quayEcosystem.Spec.Quay.EnableNodePortService {
		service.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}

	err := r.createResource(service, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayRoute(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.quayEcosystem)

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
				TargetPort: intstr.FromInt(8080),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}

	route.ObjectMeta.Labels = resources.BuildQuayResourceLabels(meta.Labels)

	if len(r.quayEcosystem.Spec.Quay.RouteHost) != 0 {
		route.Spec.Host = r.quayEcosystem.Spec.Quay.RouteHost
	}

	err := r.createResource(route, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigRoute(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayConfigResourcesName(r.quayEcosystem)

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

	route.ObjectMeta.Labels = resources.BuildQuayConfigResourceLabels(meta.Labels)

	if len(r.quayEcosystem.Spec.Quay.ConfigRouteHost) != 0 {
		route.Spec.Host = r.quayEcosystem.Spec.Quay.ConfigRouteHost
	}

	err := r.createResource(route, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) configureSCC(meta metav1.ObjectMeta) error {

	sccUser := "system:serviceaccount:" + meta.Namespace + ":" + constants.QuayEcosystemServiceAccount

	anyUIDSCC := &ossecurityv1.SecurityContextConstraints{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: constants.AnyUIDSCC, Namespace: ""}, anyUIDSCC)

	if err != nil {
		logrus.Errorf("Error occurred retrieving SCC: %v", err)
		return err
	}

	sccUserFound := false
	for _, user := range anyUIDSCC.Users {
		if user == sccUser {

			sccUserFound = true
			break
		}
	}

	if !sccUserFound {
		anyUIDSCC.Users = append(anyUIDSCC.Users, sccUser)
		err = r.client.Update(context.TODO(), anyUIDSCC)
		if err != nil {
			return err
		}
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayRegistryStorage(meta metav1.ObjectMeta) error {
	meta.Name = resources.GetQuayRegistryStorageName(r.quayEcosystem)

	registryStoragePVC := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: r.quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.AccessModes,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(r.quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.Capacity),
				},
			},
		},
	}

	err := r.createResource(registryStoragePVC, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayDeployment(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.quayEcosystem)
	resources.BuildQuayResourceLabels(meta.Labels)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: r.quayEcosystem.Spec.Quay.Image,
			Name:  meta.Name,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
				Name:          "http",
			}, {
				ContainerPort: 8080,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      "configvolume",
				MountPath: "/conf/stack",
				ReadOnly:  false,
			}},
		}},
		ServiceAccountName: constants.QuayEcosystemServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: "configvolume",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: resources.GetConfigMapSecretName(r.quayEcosystem),
								},
							},
						},
					},
				},
			},
		}},
	}

	if len(r.quayEcosystem.Spec.ImagePullSecretName) != 0 {
		quayDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: r.quayEcosystem.Spec.ImagePullSecretName,
		},
		}
	}

	if !reflect.DeepEqual(copv1alpha1.RegistryStorage{}, r.quayEcosystem.Spec.Quay.RegistryStorage) && !reflect.DeepEqual(copv1alpha1.PersistentVolumeRegistryStorageType{}, r.quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume) {

		quayDeploymentPodSpec.Volumes = append(quayDeploymentPodSpec.Volumes, corev1.Volume{
			Name: "registryvolume",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: resources.GetQuayRegistryStorageName(r.quayEcosystem),
				},
			},
		})
		quayDeploymentPodSpec.Containers[0].VolumeMounts = append(quayDeploymentPodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "registryvolume",
			MountPath: r.quayEcosystem.Spec.Quay.RegistryStorage.StorageDirectory,
			ReadOnly:  false,
		})
	}

	quayDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: r.quayEcosystem.Spec.Quay.Replicas,
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

	err := r.createResource(quayDeployment, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayConfigDeployment(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayConfigResourcesName(r.quayEcosystem)
	resources.BuildQuayConfigResourceLabels(meta.Labels)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: r.quayEcosystem.Spec.Quay.Image,
			Name:  meta.Name,
			Env: []corev1.EnvVar{
				{
					Name:  constants.QuayEntryName,
					Value: constants.QuayEntryConfigValue,
				},
				{
					Name:  constants.QuayConfigPasswordName,
					Value: "quay",
				},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
				Name:          "http",
			}, {
				ContainerPort: 8080,
				Name:          "https",
			}},
			VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
				Name:      "configvolume",
				MountPath: "/conf/stack",
				ReadOnly:  false,
			}},
		}},
		ServiceAccountName: constants.QuayEcosystemServiceAccount,
		Volumes: []corev1.Volume{corev1.Volume{
			Name: "configvolume",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: resources.GetConfigMapSecretName(r.quayEcosystem),
								},
							},
						},
					},
				},
			}}},
	}

	if len(r.quayEcosystem.Spec.ImagePullSecretName) != 0 {
		quayDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: r.quayEcosystem.Spec.ImagePullSecretName,
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

	err := r.createResource(quayDeployment, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRedisService(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetRedisResourcesName(r.quayEcosystem)

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

	service.ObjectMeta.Labels = resources.BuildRedisResourceLabels(meta.Labels)

	err := r.createResource(service, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) redisDeployment(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetRedisResourcesName(r.quayEcosystem)

	redisDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: r.quayEcosystem.Spec.Redis.Image,
			Name:  meta.Name,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 6379,
			}},
		}},
		ServiceAccountName: constants.QuayEcosystemServiceAccount,
	}

	if len(r.quayEcosystem.Spec.ImagePullSecretName) != 0 {
		redisDeploymentPodSpec.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{
			Name: r.quayEcosystem.Spec.ImagePullSecretName,
		},
		}
	}

	redisDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: r.quayEcosystem.Spec.Quay.Replicas,
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

	err := r.createResource(redisDeployment, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createResource(obj metav1.Object, quayEcosystem *copv1alpha1.QuayEcosystem) error {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object", obj)
	}

	// Set Quay instance as the owner and controller
	if err := controllerutil.SetControllerReference(quayEcosystem, obj, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), runtimeObj)
}

func (r *ReconcileQuayEcosystemConfiguration) updateResource(obj metav1.Object) error {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object", obj)
	}

	// set QuayEcosystem instance as the owner and controller
	_ = controllerutil.SetControllerReference(r.quayEcosystem, obj, r.scheme)

	return r.client.Update(context.TODO(), runtimeObj)
}

func (r *ReconcileQuayEcosystemConfiguration) createOrUpdateResource(obj metav1.Object) error {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object", obj)
	}

	// set QuayEcosystem instance as the owner and controller, don't check error(can be already set)
	_ = controllerutil.SetControllerReference(r.quayEcosystem, obj, r.scheme)

	err := r.client.Create(context.TODO(), runtimeObj)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return r.updateResource(obj)
	} else if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
