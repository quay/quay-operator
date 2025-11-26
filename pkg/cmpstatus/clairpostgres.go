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

// ClairPostgres checks a quay registry clairpostgres component status. In order to evaluate the status for the
// clair component we need to verify if clairpostgres succeed.
type ClairPostgres struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (c *ClairPostgres) Name() string {
	return "clairpostgres"
}

// Check verifies if the clairpostgres deployment associated with provided quay registry
// were created and rolled out as expected.
func (c *ClairPostgres) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentClairPostgres) {
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "ClairPostgres not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	depname := fmt.Sprintf("%s-%s", reg.Name, "clair-postgres")
	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      depname,
	}

	var dep appsv1.Deployment
	if err := c.Client.Get(ctx, nsn, &dep); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("Deployment %s not found", depname)
			return qv1.Condition{
				Type:           qv1.ComponentClairPostgresReady,
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
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	cond := c.deploy.check(dep)
	if cond.Status != metav1.ConditionTrue {
		// if the deployment is in a faulty state bails out immediately.
		cond.Type = qv1.ComponentClairPostgresReady
		return cond, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentClairPostgresReady,
		Reason:         qv1.ConditionReasonComponentReady,
		Status:         metav1.ConditionTrue,
		Message:        "ClairPostgres component healthy",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
