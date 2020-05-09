package resources

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetSCCRoleDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *rbacv1.Role {
	meta.Name = GetSCCResourcesName(quayEcosystem)

	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{constants.AnyUIDSCC},
				Verbs:         []string{"use"},
			},
		},
	}
}

func GetQuayRoleDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *rbacv1.Role {

	meta.Name = GetQuayResourcesName(quayEcosystem)

	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "patch", "update", "create"},
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
}

func GetQuayRoleBindingDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem) *rbacv1.RoleBinding {

	meta.Name = GetQuayResourcesName(quayEcosystem)

	return &rbacv1.RoleBinding{
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
				Name:      constants.QuayServiceAccount,
				Namespace: meta.Namespace,
			},
		},
	}

}

func GetSCCRoleBindingDefinition(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem, sccSAs []string) *rbacv1.RoleBinding {

	meta.Name = GetSCCResourcesName(quayEcosystem)

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
		Subjects: []rbacv1.Subject{},
	}

	for _, sa := range sccSAs {

		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      sa,
			Namespace: meta.Namespace,
		})
	}

	return roleBinding

}
