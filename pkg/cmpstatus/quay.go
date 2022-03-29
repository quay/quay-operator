package cmpstatus

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Quay checks a quay registry base component status. In order to evaluate the status for the
// base component we need to verify if quay and config-editor deployments succeed.
type Quay struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (q *Quay) Name() string {
	return "quay"
}

// Check verifies if the quay and config-editor deployment associated with provided quay registry
// were created and rolled out as expected.
func (q *Quay) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	// check the upgrade job status. this job is created as part of "quay component" and
	// must succeed for quay deployment to rollout.
	if err := q.upgradeJob(ctx, reg); err != nil {
		return qv1.Condition{
			Type:           qv1.ComponentQuayReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        err.Error(),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// users are able to override the number of replicas. if they do override it to zero
	// we expect zero replicas to be running.
	replicas := qv1.GetReplicasOverrideForComponent(&reg, qv1.ComponentQuay)
	scaleddown := replicas != nil && *replicas == 0

	// we need to check two distinct deployments, the quay app and its config editor.
	for _, depsuffix := range []string{"quay-app", "quay-config-editor"} {
		depname := fmt.Sprintf("%s-%s", reg.Name, depsuffix)
		nsn := types.NamespacedName{
			Namespace: reg.Namespace,
			Name:      depname,
		}

		var dep appsv1.Deployment
		if err := q.Client.Get(ctx, nsn, &dep); err != nil {
			if errors.IsNotFound(err) {
				msg := fmt.Sprintf("Deployment %s not found", depname)
				return qv1.Condition{
					Type:           qv1.ComponentQuayReady,
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
				Type:           qv1.ComponentQuayReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        msg,
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		basedep := fmt.Sprintf("%s-quay-app", reg.Name)
		if dep.Name == basedep && scaleddown {
			// if user has scaled down base component and we have zero replicas for
			// the current deployment we are good to go, move to the next deployment.
			if dep.Status.AvailableReplicas == 0 {
				continue
			}

			return qv1.Condition{
				Type:           qv1.ComponentQuayReady,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Status:         metav1.ConditionFalse,
				Message:        "Quay component is being scaled down",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		cond := q.deploy.check(dep)
		if cond.Status != metav1.ConditionTrue {
			// if the deployment is in a faulty state bails out immediately.
			cond.Type = qv1.ComponentQuayReady
			return cond, nil
		}
	}

	return qv1.Condition{
		Type:           qv1.ComponentQuayReady,
		Reason:         qv1.ConditionReasonComponentReady,
		Status:         metav1.ConditionTrue,
		Message:        "Quay component healthy",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}

// upgradeJob checks the status for the upgrade job created as part of the Quay component. If
// this function returns an error this error can be used as status for the Quay component.
func (q *Quay) upgradeJob(ctx context.Context, reg qv1.QuayRegistry) error {
	jname := fmt.Sprintf("%s-quay-app-upgrade", reg.Name)
	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      jname,
	}

	var job batchv1.Job
	if err := q.Client.Get(ctx, nsn, &job); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Job %s not found", jname)
		}
		return fmt.Errorf("unexpected error reading upgrade job: %w", err)
	}

	if job.Status.Succeeded == 0 {
		return fmt.Errorf("Job %s not finished", jname)
	}
	return nil
}
