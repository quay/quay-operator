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

// Postgres checks a quay registry postgres component status.
type Postgres struct {
	Client client.Client
	deploy deploy
}

// Name returns the component name this entity checks for health.
func (p *Postgres) Name() string {
	return "postgres"
}

// Check verifies if the postgres deployment associated with provided quay registry was created
// and rolled out as expected, also checking its PVC status dynamically.
func (p *Postgres) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentPostgres) {
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Postgres not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	deploymentName := fmt.Sprintf("%s-quay-database", reg.Name)
	nsnDeployment := types.NamespacedName{Namespace: reg.Namespace, Name: deploymentName}

	var dep appsv1.Deployment
	if err := p.Client.Get(ctx, nsnDeployment, &dep); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentPostgresReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        fmt.Sprintf("Postgres deployment %s not found", deploymentName),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, fmt.Errorf("failed to get Postgres deployment %s: %w", deploymentName, err)
	}

	if !qv1.Owns(reg, &dep) {
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        fmt.Sprintf("Postgres deployment %s not owned by QuayRegistry", deploymentName),
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
		// This case should ideally not happen for Postgres if it's configured to use a PVC.
		// If it does, the deployment readiness check will likely fail, or we can return a specific error.
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        fmt.Sprintf("Postgres deployment %s does not reference a PersistentVolumeClaim", deploymentName),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	nsnPVC := types.NamespacedName{Namespace: reg.Namespace, Name: pvcName}
	var pvc corev1.PersistentVolumeClaim
	if err := p.Client.Get(ctx, nsnPVC, &pvc); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentPostgresReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        fmt.Sprintf("Postgres PersistentVolumeClaim %s (referenced by deployment %s) not found", pvcName, deploymentName),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, fmt.Errorf("failed to get Postgres PVC %s: %w", pvcName, err)
	}

	if pvc.Status.Phase == corev1.ClaimPending {
		var eventList corev1.EventList
		if err := p.Client.List(ctx, &eventList, client.InNamespace(reg.Namespace), &client.MatchingFields{"involvedObject.uid": string(pvc.UID)}); err == nil {
			for _, event := range eventList.Items {
				if event.Type == corev1.EventTypeWarning && (event.Reason == "ProvisioningFailed" || event.Reason == "FailedBinding") {
					return qv1.Condition{
						Type:           qv1.ComponentPostgresReady,
						Status:         metav1.ConditionFalse,
						Reason:         qv1.ConditionReasonPVCProvisioningFailed,
						Message:        fmt.Sprintf("Postgres PVC %s provisioning failed: %s", pvc.Name, event.Message),
						LastUpdateTime: metav1.NewTime(time.Now()),
					}, nil
				}
			}
		}
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonPVCPending,
			Message:        fmt.Sprintf("Postgres PersistentVolumeClaim %s is pending", pvc.Name),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	if pvc.Status.Phase != corev1.ClaimBound {
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonPVCPending,
			Message:        fmt.Sprintf("Postgres PersistentVolumeClaim %s is not bound (current phase: %s)", pvc.Name, pvc.Status.Phase),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// If PVC is bound, then check deployment readiness
	cond := p.deploy.check(dep)
	cond.Type = qv1.ComponentPostgresReady
	return cond, nil
}
