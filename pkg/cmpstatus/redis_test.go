package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestRedisCheck(t *testing.T) {
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
							Kind:    qv1.ComponentRedis,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRedisReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Redis not managed by the operator",
			},
		},
		{
			name: "deployment not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRedisReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Redis deployment not found",
			},
		},
		{
			name: "deployment not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-redis",
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRedisReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Redis deployment not owned by QuayRegistry",
			},
		},
		{
			name: "deployment not available",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
					},
				},
			},
			objs: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-redis",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Message: "something went wrong",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRedisReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-quay-redis: something went wrong",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewFakeClient(tt.objs...)
			redis := Redis{
				Client: cli,
			}

			cond, err := redis.Check(ctx, tt.quay)
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
