package cmpstatus

import (
	"context"
	"time"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cache checks the status of the cache component.
type Cache struct {
	Client client.Client
}

// Name provides the canonical name of the component.
func (c *Cache) Name() string {
	return "cache"
}

// Check verifies that the cache component is present in the QuayRegistry custom resource. We are only interested
// if the component is managed or not.
func (c *Cache) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentCache) {
		return qv1.Condition{
			Type:           qv1.ComponentCacheReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Cache component is not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentRedis) {
		return qv1.Condition{
			Type:           qv1.ComponentCacheReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonCacheComponentDependencyError,
			Message:        "Cache component managed, but Redis is unmanaged",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentCacheReady,
		Status:         metav1.ConditionTrue,
		Reason:         qv1.ConditionReasonComponentReady,
		Message:        "Cache component found",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
