package cmpstatus

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// ClairPostgres checks a quay registry clairpostgres component status.
type ClairPostgres struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (c *ClairPostgres) Name() string {
	return "clairpostgres"
}

// Check verifies if the clairpostgres deployment associated with provided quay registry
// was created and rolled out as expected, also checking its PVC status dynamically.
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

	deploymentName := fmt.Sprintf("%s-%s", reg.Name, "clair-postgres")
	nsnDeployment := types.NamespacedName{Namespace: reg.Namespace, Name: deploymentName}

	var dep appsv1.Deployment
	if err := c.Client.Get(ctx, nsnDeployment, &dep); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentClairPostgresReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        fmt.Sprintf("ClairPostgres deployment %s not found", deploymentName),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, fmt.Errorf("failed to get ClairPostgres deployment %s: %w", deploymentName, err)
	}

	if !qv1.Owns(reg, &dep) {
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        fmt.Sprintf("ClairPostgres deployment %s not owned by QuayRegistry", deploymentName),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// Dynamically find the PVC name from the deployment's volumes
	var pvcName string
	for _, vol := range dep.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcName = vol.PersistentVolumeClaim.ClaimName
			break
		}
	}

	if pvcName == "" {
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        fmt.Sprintf("ClairPostgres deployment %s does not reference a PersistentVolumeClaim", deploymentName),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	nsnPVC := types.NamespacedName{Namespace: reg.Namespace, Name: pvcName}
	var pvc corev1.PersistentVolumeClaim
	if err := c.Client.Get(ctx, nsnPVC, &pvc); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentClairPostgresReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        fmt.Sprintf("ClairPostgres PersistentVolumeClaim %s (referenced by deployment %s) not found", pvcName, deploymentName),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, fmt.Errorf("failed to get ClairPostgres PVC %s: %w", pvcName, err)
	}

	if pvc.Status.Phase == corev1.ClaimPending {
		var eventList corev1.EventList
		if err := c.Client.List(ctx, &eventList, client.InNamespace(reg.Namespace), &client.MatchingFields{"involvedObject.uid": string(pvc.UID)}); err == nil {
			for _, event := range eventList.Items {
				if event.Type == corev1.EventTypeWarning && (event.Reason == "ProvisioningFailed" || event.Reason == "FailedBinding") {
					return qv1.Condition{
						Type:           qv1.ComponentClairPostgresReady,
						Status:         metav1.ConditionFalse,
						Reason:         qv1.ConditionReasonPVCProvisioningFailed,
						Message:        fmt.Sprintf("ClairPostgres PVC %s provisioning failed: %s", pvc.Name, event.Message),
						LastUpdateTime: metav1.NewTime(time.Now()),
					}, nil
				}
			}
		}
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonPVCPending,
			Message:        fmt.Sprintf("ClairPostgres PersistentVolumeClaim %s is pending", pvc.Name),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	if pvc.Status.Phase != corev1.ClaimBound {
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonPVCPending,
			Message:        fmt.Sprintf("ClairPostgres PersistentVolumeClaim %s is not bound (current phase: %s)", pvc.Name, pvc.Status.Phase),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	cond := c.deploy.check(dep)
	cond.Type = qv1.ComponentClairPostgresReady
	if cond.Status == metav1.ConditionTrue && cond.Reason == qv1.ConditionReasonComponentReady {
		cond.Message = "ClairPostgres component healthy"
	}
	return cond, nil
}
