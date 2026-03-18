package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func newUnstructuredPrometheusRule(name string, ownerRefs []metav1.OwnerReference) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "PrometheusRule",
	})
	obj.SetName(name)
	if len(ownerRefs) > 0 {
		obj.SetOwnerReferences(ownerRefs)
	}
	return obj
}

func newUnstructuredServiceMonitor(name string, ownerRefs []metav1.OwnerReference) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	})
	obj.SetName(name)
	if len(ownerRefs) > 0 {
		obj.SetOwnerReferences(ownerRefs)
	}
	return obj
}

func TestMonitoringCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
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
			objs: []client.Object{
				newUnstructuredPrometheusRule("registry-quay-prometheus-rules", nil),
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
			objs: []client.Object{
				newUnstructuredPrometheusRule("registry-quay-prometheus-rules", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
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
			objs: []client.Object{
				newUnstructuredPrometheusRule("registry-quay-prometheus-rules", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
				newUnstructuredServiceMonitor("registry-quay-metrics-monitor", nil),
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
			objs: []client.Object{
				newUnstructuredPrometheusRule("registry-quay-prometheus-rules", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
				newUnstructuredServiceMonitor("registry-quay-metrics-monitor", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
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
			builder := fake.NewClientBuilder()
			cli := builder.WithObjects(tt.objs...).WithScheme(scheme).Build()
			mon := Monitoring{cli}

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
