package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCacheCheck(t *testing.T) {
	var checkCacheTest = []struct {
		name string
		objs []client.Object
		quay qv1.QuayRegistry
		cond qv1.Condition
	}{
		{
			name: "cache_and_redis_managed",
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
						{
							Kind:    qv1.ComponentCache,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentCacheReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Cache component found",
			},
		},
		{
			name: "cache_not_managed",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentCache,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentCacheReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Cache component is not managed by the operator",
			},
		},
		{
			name: "cache_managed_but_redis_unmanaged",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentCache,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRedis,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentCacheReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonCacheComponentDependencyError,
				Message: "Cache component managed, but Redis is unmanaged",
			},
		},
	}

	for _, tt := range checkCacheTest {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			cache := Cache{
				Client: cli,
			}

			cond, err := cache.Check(ctx, tt.quay)
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
