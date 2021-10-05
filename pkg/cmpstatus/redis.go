package cmpstatus

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Redis checks a quay registry Redis component status.
type Redis struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (r *Redis) Name() string {
	return "redis"
}

// Check verifies if the redis deployment associated with provided quay registry was created and
// rolled out as expected.
func (r *Redis) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentRedis) {
		return qv1.Condition{
			Type:           qv1.ComponentRedisReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Redis not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      fmt.Sprintf("%s-quay-redis", reg.Name),
	}

	var dep appsv1.Deployment
	if err := r.Client.Get(ctx, nsn, &dep); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentRedisReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Redis deployment not found",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if !qv1.Owns(reg, &dep) {
		return qv1.Condition{
			Type:           qv1.ComponentRedisReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        "Redis deployment not owned by QuayRegistry",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	cond := r.deploy.check(dep)
	cond.Type = qv1.ComponentRedisReady
	return cond, nil
}
