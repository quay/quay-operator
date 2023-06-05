package cmpstatus

import (
	"context"
	"fmt"
	"time"

	asv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// HPA checks a quay registry HorizontalPodAutoscaler status.
type HPA struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (h *HPA) Name() string {
	return "horizontalpodautoscaler"
}

// Check verifies if the horizontal pod autoscaler was deployed by the operator. We expect to
// find one HPA owned by the provided QuayRegistry object if ComponentHPA is managed.
func (h *HPA) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentHPA) {
		return qv1.Condition{
			Type:           qv1.ComponentHPAReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Horizontal pod autoscaler not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	for _, hpasuffix := range []string{"quay-app", "clair-app", "quay-mirror"} {
		nsn := types.NamespacedName{
			Namespace: reg.Namespace,
			Name:      fmt.Sprintf("%s-%s", reg.Name, hpasuffix),
		}

		var hpa asv2.HorizontalPodAutoscaler
		if err := h.Client.Get(ctx, nsn, &hpa); err != nil {
			if errors.IsNotFound(err) {
				return qv1.Condition{
					Type:           qv1.ComponentHPAReady,
					Status:         metav1.ConditionFalse,
					Reason:         qv1.ConditionReasonComponentNotReady,
					Message:        "Horizontal pod autoscaler not found",
					LastUpdateTime: metav1.NewTime(time.Now()),
				}, nil
			}
			return zero, err
		}

		if !qv1.Owns(reg, &hpa) {
			return qv1.Condition{
				Type:           qv1.ComponentHPAReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Horizontal pod autoscaler not owned by QuayRegistry",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
	}

	return qv1.Condition{
		Type:           qv1.ComponentHPAReady,
		Status:         metav1.ConditionTrue,
		Reason:         qv1.ConditionReasonComponentReady,
		Message:        "Horizontal pod autoscaler found",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
