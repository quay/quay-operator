package configuration

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	ossecurityv1 "github.com/openshift/api/security/v1"

	routev1 "github.com/openshift/api/route/v1"
	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

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
	metaObject := NewResourceObjectMeta(r.quayEcosystem)

	if err := r.createQuayConfigSecret(metaObject); err != nil {
		return &reconcile.Result{}, err
	}

	if err := r.createRBAC(metaObject); err != nil {
		return nil, err
	}

	if err := r.createQuayEcosystemServiceAccount(metaObject); err != nil {
		return nil, err
	}

	if err := r.configureSCC(metaObject); err != nil {
		return nil, err
	}

	// Redis
	if !r.quayEcosystem.Spec.Redis.Skip {
		if err := r.createRedisService(metaObject); err != nil {
			return nil, err
		}

		if err := r.redisDeployment(metaObject); err != nil {
			return nil, err
		}

	}

	// TODO: Database (PostgreSQL/MySQL)

	// Quay Resources
	if err := r.createQuayService(metaObject); err != nil {
		return nil, err
	}

	if err := r.createQuayRoute(metaObject); err != nil {
		return nil, err
	}

	if err := r.quayDeployment(metaObject); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := GetConfigMapSecretName(r.quayEcosystem)

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

	meta.Name = GetGenericResourcesName(r.quayEcosystem)

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

	meta.Name = GetQuayResourcesName(r.quayEcosystem)

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
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}

	service.ObjectMeta.Labels = BuildQuayResourceLabels(meta.Labels)

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

	meta.Name = GetQuayResourcesName(r.quayEcosystem)

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
				TargetPort: intstr.FromInt(80),
			},
		},
	}

	route.ObjectMeta.Labels = BuildQuayResourceLabels(meta.Labels)

	if len(r.quayEcosystem.Spec.Quay.RouteHost) != 0 {
		route.Spec.Host = r.quayEcosystem.Spec.Quay.RouteHost
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

func (r *ReconcileQuayEcosystemConfiguration) quayDeployment(meta metav1.ObjectMeta) error {

	meta.Name = GetQuayResourcesName(r.quayEcosystem)

	quayDeploymentPodSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: r.quayEcosystem.Spec.Quay.Image,
			Name:  meta.Name,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 80,
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
				Secret: &corev1.SecretVolumeSource{
					SecretName: GetConfigMapSecretName(r.quayEcosystem),
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

func (r *ReconcileQuayEcosystemConfiguration) createRedisService(meta metav1.ObjectMeta) error {

	meta.Name = GetRedisResourcesName(r.quayEcosystem)

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

	err := r.createResource(service, r.quayEcosystem)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) redisDeployment(meta metav1.ObjectMeta) error {

	meta.Name = GetRedisResourcesName(r.quayEcosystem)

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

	// set Jenkins instance as the owner and controller, don't check error(can be already set)
	_ = controllerutil.SetControllerReference(r.quayEcosystem, obj, r.scheme)

	err := r.client.Create(context.TODO(), runtimeObj)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return r.updateResource(obj)
	} else if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
