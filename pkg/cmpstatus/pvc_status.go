package cmpstatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// CheckPVCStatusForDeployment checks the status of a PersistentVolumeClaim associated with a Deployment.
// It returns a condition indicating the readiness of the component.
func CheckPVCStatusForDeployment(
	ctx context.Context,
	cli client.Client,
	quay qv1.QuayRegistry,
	component qv1.ComponentKind,
	deploymentNameSuffix string,
	componentReadyType qv1.ConditionType,
) (qv1.Condition, error) {
	var zero qv1.Condition

	componentName := strings.Title(string(component))
	if component == qv1.ComponentClairPostgres {
		componentName = "ClairPostgres"
	}

	nsn := types.NamespacedName{
		Namespace: quay.Namespace,
		Name:      fmt.Sprintf("%s-%s", quay.Name, deploymentNameSuffix),
	}

	var dep appsv1.Deployment
	if err := cli.Get(ctx, nsn, &dep); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:   componentReadyType,
				Status: metav1.ConditionFalse,
				Reason: qv1.ConditionReasonComponentNotReady,
				Message: fmt.Sprintf(
					"%s deployment %s not found",
					componentName,
					nsn.Name,
				),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if !qv1.Owns(quay, &dep) {
		return qv1.Condition{
			Type:   componentReadyType,
			Status: metav1.ConditionFalse,
			Reason: qv1.ConditionReasonComponentNotReady,
			Message: fmt.Sprintf(
				"%s deployment %s not owned by QuayRegistry",
				componentName,
				nsn.Name,
			),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// Find the PVC used by the deployment
	var pvcName string
	for _, vol := range dep.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcName = vol.PersistentVolumeClaim.ClaimName
			break
		}
	}

	if pvcName == "" {
		// If no PVC is found, we assume it's not needed and check deployment status directly
		deployCheck := deploy{}
		cond := deployCheck.check(dep)
		cond.Type = componentReadyType
		return cond, nil
	}

	// Check the PVC status
	pvcNSN := types.NamespacedName{
		Namespace: quay.Namespace,
		Name:      pvcName,
	}
	var pvc corev1.PersistentVolumeClaim
	if err := cli.Get(ctx, pvcNSN, &pvc); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           componentReadyType,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        fmt.Sprintf("PersistentVolumeClaim %s not found", pvcName),
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if pvc.Status.Phase == corev1.ClaimPending {
		// If the PVC is pending, check for provisioning failure events
		var eventList corev1.EventList
		opts := []client.ListOption{
			client.InNamespace(quay.Namespace),
			client.MatchingFields{"involvedObject.uid": string(pvc.UID)},
		}
		if err := cli.List(ctx, &eventList, opts...); err != nil {
			return zero, fmt.Errorf("failed to list events for pvc %s: %w", pvc.Name, err)
		}

		for _, event := range eventList.Items {
			if event.Reason == "ProvisioningFailed" {
				return qv1.Condition{
					Type:   componentReadyType,
					Status: metav1.ConditionFalse,
					Reason: qv1.ConditionReasonPVCProvisioningFailed,
					Message: fmt.Sprintf(
						"%s PVC %s provisioning failed: %s",
						componentName,
						pvc.Name,
						event.Message,
					),
					LastUpdateTime: metav1.NewTime(time.Now()),
				}, nil
			}
		}

		return qv1.Condition{
			Type:           componentReadyType,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonPVCPending,
			Message:        fmt.Sprintf("%s PersistentVolumeClaim %s is pending", componentName, pvc.Name),
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	// If PVC is not pending, we check deployment health
	deployCheck := deploy{}
	cond := deployCheck.check(dep)
	cond.Type = componentReadyType
	return cond, nil
}
