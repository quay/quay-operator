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

// Mirror checks a quay registry mirror component status.
type Mirror struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (m *Mirror) Name() string {
	return "mirror"
}

// Check verifies if the mirror deployment associated with provided quay registry was created and
// rolled out as expected.
func (m *Mirror) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentMirror) {
		return qv1.Condition{
			Type:           qv1.ComponentMirrorReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Mirror not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      fmt.Sprintf("%s-quay-mirror", reg.Name),
	}

	var dep appsv1.Deployment
	if err := m.Client.Get(ctx, nsn, &dep); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentMirrorReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Mirror deployment not found",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if !qv1.Owns(reg, &dep) {
		return qv1.Condition{
			Type:           qv1.ComponentMirrorReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        "Mirror deployment not owned by QuayRegistry",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// users are able to override the number of replicas. if they do override it to zero
	// we expect zero replicas to be running.
	replicas := qv1.GetReplicasOverrideForComponent(&reg, qv1.ComponentMirror)
	scaleddown := replicas != nil && *replicas == 0
	if scaleddown {
		if dep.Status.AvailableReplicas == 0 {
			return qv1.Condition{
				Type:           qv1.ComponentMirrorReady,
				Reason:         qv1.ConditionReasonComponentReady,
				Status:         metav1.ConditionTrue,
				Message:        "Mirror manually scaled down",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentMirrorReady,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Status:         metav1.ConditionFalse,
			Message:        "Mirror component is being scaled down",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	cond := m.deploy.check(dep)
	cond.Type = qv1.ComponentMirrorReady
	return cond, nil
}
