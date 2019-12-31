package externalaccess

import (
	"context"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
)

type RouteExternalAccess struct {
	QuayConfiguration *resources.QuayConfiguration
	ReconcilerBase    util.ReconcilerBase
}

func (r *RouteExternalAccess) ManageQuayExternalAccess(meta metav1.ObjectMeta) error {
	meta.Name = resources.GetQuayResourcesName(r.QuayConfiguration.QuayEcosystem)

	route := resources.GetQuayRouteDefinition(meta, r.QuayConfiguration.QuayEcosystem)

	err := r.ReconcilerBase.CreateOrUpdateResource(r.QuayConfiguration.QuayEcosystem, r.QuayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	createdRoute := &routev1.Route{}
	err = r.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: meta.Name, Namespace: r.QuayConfiguration.QuayEcosystem.Namespace}, createdRoute)

	if err != nil {
		return err
	}

	if utils.IsZeroOfUnderlyingType(r.QuayConfiguration.QuayEcosystem.Spec.Quay.Hostname) {
		r.QuayConfiguration.QuayHostname = createdRoute.Spec.Host
	} else {
		r.QuayConfiguration.QuayHostname = r.QuayConfiguration.QuayEcosystem.Spec.Quay.Hostname
	}

	r.QuayConfiguration.QuayEcosystem.Status.Hostname = r.QuayConfiguration.QuayHostname

	return nil

}

func (r *RouteExternalAccess) ManageQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	route := resources.GetQuayConfigRouteDefinition(meta, r.QuayConfiguration.QuayEcosystem)

	err := r.ReconcilerBase.CreateOrUpdateResource(r.QuayConfiguration.QuayEcosystem, r.QuayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	return nil

}

func (r *RouteExternalAccess) RemoveQuayConfigExternalAccess(meta metav1.ObjectMeta) error {

	quayName := resources.GetQuayConfigResourcesName(r.QuayConfiguration.QuayEcosystem)

	route := &routev1.Route{}
	err := r.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: quayName, Namespace: r.QuayConfiguration.QuayEcosystem.Namespace}, route)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Finding Quay Config Route", "Namespace", r.QuayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return err
	}

	err = r.ReconcilerBase.GetClient().Delete(context.TODO(), route)

	return nil

}
