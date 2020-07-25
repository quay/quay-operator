package externalaccess

import (
	"context"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type IngressExternalAccess struct {
	QuayConfiguration *resources.QuayConfiguration
	ReconcilerBase    util.ReconcilerBase
}

func (r IngressExternalAccess) ManageQuayExternalAccess(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.QuayConfiguration.QuayEcosystem)

	err := r.configureIngress(meta, r.QuayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.Hostname, false)

	if err != nil {
		return nil
	}

	r.QuayConfiguration.QuayHostname = r.QuayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.Hostname

	r.QuayConfiguration.QuayEcosystem.Status.Hostname = r.QuayConfiguration.QuayHostname

	return nil
}

func (r *IngressExternalAccess) ManageQuayConfigExternalAccess(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayConfigResourcesName(r.QuayConfiguration.QuayEcosystem)

	if !utils.IsZeroOfUnderlyingType(r.QuayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.ConfigHostname) {
		r.QuayConfiguration.QuayConfigHostname = r.QuayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.ConfigHostname
		return r.configureIngress(meta, r.QuayConfiguration.QuayEcosystem.Spec.Quay.ExternalAccess.ConfigHostname, true)
	} else {
		r.QuayConfiguration.QuayConfigHostname = meta.Name
		return nil
	}

}

func (r *IngressExternalAccess) RemoveQuayConfigExternalAccess(meta metav1.ObjectMeta) error {

	quayName := resources.GetQuayConfigResourcesName(r.QuayConfiguration.QuayEcosystem)

	ingress := &networkingv1beta1.Ingress{}
	err := r.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: quayName, Namespace: r.QuayConfiguration.QuayEcosystem.Namespace}, ingress)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Finding Quay Config Ingress", "Namespace", r.QuayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return err
	}

	err = r.ReconcilerBase.GetClient().Delete(context.TODO(), ingress)

	return nil
}

func (r *IngressExternalAccess) configureIngress(meta metav1.ObjectMeta, hostname string, isQuayConfig bool) error {

	ingress := resources.GetQuayIngressDefinition(meta, r.QuayConfiguration.QuayEcosystem, hostname, r.QuayConfiguration.QuayTLSSecretName, isQuayConfig)

	if isQuayConfig {
		ingress.ObjectMeta.Labels = resources.BuildQuayConfigResourceLabels(meta.Labels)
	} else {
		ingress.ObjectMeta.Labels = resources.BuildQuayResourceLabels(meta.Labels)
	}

	err := r.ReconcilerBase.CreateOrUpdateResource(r.QuayConfiguration.QuayEcosystem, r.QuayConfiguration.QuayEcosystem.Namespace, ingress)

	if err != nil {
		return err
	}

	return nil

}
