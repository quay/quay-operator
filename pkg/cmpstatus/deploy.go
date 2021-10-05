package cmpstatus

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// deploy checks the status for a given deployment instance. This type is used by components that
// create objects of type Deployment when they need to ensure if a deployment rolled out just fine.
// XXX this could as well be just a function but for sake of keeping everything similar in this
// package this got promoted to an empty struct.
type deploy struct{}

// check verifies a Deployment available condition. It returns a qv1.Condition where the Type
// property is not set. Type property therefore must be set by the caller when defining the
// component the provided deployment is part of.
func (d *deploy) check(dep appsv1.Deployment) qv1.Condition {
	cond, found := d.availableCondition(dep.Status.Conditions)
	if !found {
		return qv1.Condition{
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			LastUpdateTime: metav1.NewTime(time.Now()),
			Message: fmt.Sprintf(
				"Available condition not found for %s", dep.Name,
			),
		}
	}

	if cond.Status != corev1.ConditionTrue {
		msg := fmt.Sprintf("Deployment %s: %s", dep.Name, cond.Message)
		return qv1.Condition{
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now()),
		}
	}

	return qv1.Condition{
		Status:         metav1.ConditionTrue,
		Reason:         qv1.ConditionReasonComponentReady,
		Message:        fmt.Sprintf("Deployment %s healthy", dep.Name),
		LastUpdateTime: metav1.NewTime(time.Now()),
	}
}

// availableCondition filters the provided list of conditions and returns the Available condition
// if found. Returns a boolean indicating if the Available condition was found or not.
func (d *deploy) availableCondition(
	conds []appsv1.DeploymentCondition,
) (appsv1.DeploymentCondition, bool) {
	for _, cond := range conds {
		if cond.Type != appsv1.DeploymentAvailable {
			continue
		}
		return cond, true
	}
	return appsv1.DeploymentCondition{}, false
}
