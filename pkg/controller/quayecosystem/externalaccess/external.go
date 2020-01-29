package externalaccess

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalAccess represents a contract for host to interact with mechanisms for access from external resources
type ExternalAccess interface {
	ManageQuayExternalAccess(metaObject metav1.ObjectMeta) error
	ManageQuayConfigExternalAccess(metaObject metav1.ObjectMeta) error
	RemoveQuayConfigExternalAccess(metaObject metav1.ObjectMeta) error
}
