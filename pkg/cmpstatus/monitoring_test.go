package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestMonitoringCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []runtime.Object
		cond qv1.Condition
	}{
		{
			name: "not managed",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Monitoring not managed by the operator",
			},
		},
		{
			name: "prometheus rules not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "PrometheusRule registry-quay-prometheus-rules not found",
			},
		},
		{
			name: "prometheus rule not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "PrometheusRule registry-quay-prometheus-rules not owned by QuayRegistry",
			},
		},
		{
			name: "service monitor not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "ServiceMonitor registry-quay-metrics-monitor not found",
			},
		},
		{
			name: "service monitor not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
				},
				&monv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-metrics-monitor",
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "ServiceMonitor registry-quay-metrics-monitor not owned by QuayRegistry",
			},
		},
		{
			name: "all working",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
				},
				&monv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-metrics-monitor",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentMonitoringReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "ServiceMonitor and PrometheusRules created",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()
			if err := monv1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding monitoring to scheme: %s", err)
			}

			mon := Monitoring{
				Client: fake.NewFakeClientWithScheme(scheme, tt.objs...),
			}

			cond, err := mon.Check(ctx, tt.quay)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if cond.LastUpdateTime.IsZero() {
				t.Errorf("unexpected zeroed last update time for condition")
			}

			cond.LastUpdateTime = metav1.NewTime(time.Time{})
			if !reflect.DeepEqual(tt.cond, cond) {
				t.Errorf("expecting %+v, received %+v", tt.cond, cond)
			}
		})
	}
}
