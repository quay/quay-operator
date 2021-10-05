package cmpstatus

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	routev1 "github.com/openshift/api/route/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Route checks a quay registry route status.
type Route struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (r *Route) Name() string {
	return "route"
}

// Check verifies if the managed route for a quay registry has been admitted by the ingress
// controller. Expects to find one route owned by the registry if route component is managed.
func (r *Route) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentRoute) {
		return qv1.Condition{
			Type:           qv1.ComponentRouteReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Route is not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	var list routev1.RouteList
	if err := r.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return zero, err
	}

	for _, rt := range list.Items {
		if !qv1.Owns(reg, &rt) {
			continue
		}

		if len(rt.Status.Ingress) == 0 {
			return qv1.Condition{
				Type:           qv1.ComponentRouteReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Route found but no ingress",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		var failed bool
		for _, ingress := range rt.Status.Ingress {
			ingcond := r.ingressAdmittedCondition(ingress.Conditions)
			if ingcond.Status != corev1.ConditionTrue {
				failed = true
				break
			}
		}

		if failed {
			return qv1.Condition{
				Type:           qv1.ComponentRouteReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Route not fully admitted",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentRouteReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentReady,
			Message:        "Route admitted",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentRouteReady,
		Status:         metav1.ConditionFalse,
		Reason:         qv1.ConditionReasonComponentNotReady,
		Message:        "Route not found",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}

// ingressAdmittedCondition looks and returns the Admitted condition among provided list of
// ingress conditions. Returns a bool indicating if the Admitted was found.
func (r *Route) ingressAdmittedCondition(
	conds []routev1.RouteIngressCondition,
) routev1.RouteIngressCondition {
	var zero routev1.RouteIngressCondition
	for _, cond := range conds {
		if cond.Type != routev1.RouteAdmitted {
			continue
		}
		return cond
	}
	return zero
}
