package externalaccess

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodePortExternalAccess struct{}

func (r *NodePortExternalAccess) ManageQuayExternalAccess(meta metav1.ObjectMeta) error {
	return nil
}

func (r *NodePortExternalAccess) ManageQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	return nil
}

func (r *NodePortExternalAccess) RemoveQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	return nil
}
