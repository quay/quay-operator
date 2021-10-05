package cmpstatus

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Monitoring foo bar
type Monitoring struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (m *Monitoring) Name() string {
	return "monitoring"
}

// Check verifies foo and bar
func (m *Monitoring) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentMonitoring) {
		return qv1.Condition{
			Type:           qv1.ComponentMonitoringReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Monitoring not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	prname := fmt.Sprintf("%s-quay-prometheus-rules", reg.Name)
	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      prname,
	}

	var pr monv1.PrometheusRule
	if err := m.Client.Get(ctx, nsn, &pr); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("PrometheusRule %s not found", prname)
			return qv1.Condition{
				Type:           qv1.ComponentMonitoringReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        msg,
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	if !qv1.Owns(reg, &pr) {
		msg := fmt.Sprintf("PrometheusRule %s not owned by QuayRegistry", prname)
		return qv1.Condition{
			Type:           qv1.ComponentMonitoringReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	smname := fmt.Sprintf("%s-quay-metrics-monitor", reg.Name)
	nsn = types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      smname,
	}

	var sm monv1.ServiceMonitor
	if err := m.Client.Get(ctx, nsn, &sm); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("ServiceMonitor %s not found", smname)
			return qv1.Condition{
				Type:           qv1.ComponentMonitoringReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        msg,
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, nil
	}

	if !qv1.Owns(reg, &sm) {
		msg := fmt.Sprintf("ServiceMonitor %s not owned by QuayRegistry", smname)
		return qv1.Condition{
			Type:           qv1.ComponentMonitoringReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentMonitoringReady,
		Status:         metav1.ConditionTrue,
		Reason:         qv1.ConditionReasonComponentReady,
		Message:        "ServiceMonitor and PrometheusRules created",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
