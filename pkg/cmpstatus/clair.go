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

// Clair checks a quay registry clair component status. In order to evaluate the status for the
// clair component we need to verify if clair and it database deployments succeed.
type Clair struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (c *Clair) Name() string {
	return "clair"
}

// Check verifies if the clair and its database deployment associated with provided quay registry
// were created and rolled out as expected.
func (c *Clair) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentClair) {
		return qv1.Condition{
			Type:           qv1.ComponentClairReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Clair not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	depname := fmt.Sprintf("%s-%s", reg.Name, "clair-app")
	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      depname,
	}

	var dep appsv1.Deployment
	if err := c.Client.Get(ctx, nsn, &dep); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("Deployment %s not found", depname)
			return qv1.Condition{
				Type:           qv1.ComponentClairReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        msg,
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if !qv1.Owns(reg, &dep) {
		msg := fmt.Sprintf("Deployment %s not owned by QuayRegistry", depname)
		return qv1.Condition{
			Type:           qv1.ComponentClairReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// users are able to override the number of replicas. if they do override it to zero
	// we expect zero replicas to be running.
	replicas := qv1.GetReplicasOverrideForComponent(&reg, qv1.ComponentClair)
	scaleddown := replicas != nil && *replicas == 0
	if scaleddown {
		if dep.Status.AvailableReplicas == 0 {
			return qv1.Condition{
				Type:           qv1.ComponentClairReady,
				Reason:         qv1.ConditionReasonComponentReady,
				Status:         metav1.ConditionTrue,
				Message:        "Clair manually scaled down",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentClairReady,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Status:         metav1.ConditionFalse,
			Message:        "Clair component is being scaled down",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	cond := c.deploy.check(dep)
	if cond.Status != metav1.ConditionTrue {
		// if the deployment is in a faulty state bails out immediately.
		cond.Type = qv1.ComponentClairReady
		return cond, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentClairReady,
		Reason:         qv1.ConditionReasonComponentReady,
		Status:         metav1.ConditionTrue,
		Message:        "Clair component healthy",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
