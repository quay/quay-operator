package externalaccess

import (
	"context"
	"fmt"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
)

type NodePortExternalAccess struct {
	QuayConfiguration *resources.QuayConfiguration
	ReconcilerBase    util.ReconcilerBase
}

func (r *NodePortExternalAccess) ManageQuayExternalAccess(meta metav1.ObjectMeta) error {

	serviceName := resources.GetQuayResourcesName(r.QuayConfiguration.QuayEcosystem)

	nodePort, err := r.getNodePortForService(serviceName)

	if err != nil {
		return err
	}

	r.QuayConfiguration.QuayHostname = r.formatHostname(r.QuayConfiguration.QuayHostname, nodePort)

	return nil
}

func (r *NodePortExternalAccess) ManageQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	serviceName := resources.GetQuayConfigResourcesName(r.QuayConfiguration.QuayEcosystem)

	nodePort, err := r.getNodePortForService(serviceName)

	if err != nil {
		return err
	}

	r.QuayConfiguration.QuayConfigHostname = r.formatHostname(r.QuayConfiguration.QuayConfigHostname, nodePort)

	return nil

}

func (r *NodePortExternalAccess) RemoveQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	return nil
}

func (r *NodePortExternalAccess) getNodePortForService(serviceName string) (*int32, error) {

	service := &corev1.Service{}
	err := r.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: r.QuayConfiguration.QuayEcosystem.Namespace}, service)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Finding Service", "Namespace", r.QuayConfiguration.QuayEcosystem.Namespace, "Name", serviceName)
		return nil, err
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return nil, fmt.Errorf("Unexpected type for service. Service Name: %s, Expected Type: %s, Actual Type: %s", serviceName, corev1.ServiceTypeNodePort, service.Spec.Type)
	}

	return &service.Spec.Ports[0].NodePort, nil

}

func (r *NodePortExternalAccess) formatHostname(hostname string, nodePort *int32) string {
	return fmt.Sprintf("%s:%s", hostname, fmt.Sprint(*nodePort))
}
