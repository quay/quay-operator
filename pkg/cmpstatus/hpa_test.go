package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	asv2b2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestHPACheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "unmanaged",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentHPA,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentHPAReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Horizontal pod autoscaler not managed by the operator",
			},
		},
		{
			name: "hpa not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentHPA,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentHPAReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Horizontal pod autoscaler not found",
			},
		},
		{
			name: "hpa found but not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentHPA,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&asv2b2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app",
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentHPAReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Horizontal pod autoscaler not owned by QuayRegistry",
			},
		},
		{
			name: "hpa found and owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentHPA,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&asv2b2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app",
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
				&asv2b2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-mirror",
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
				&asv2b2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-clair-app",
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
				Type:    qv1.ComponentHPAReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Horizontal pod autoscaler found",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			hpa := HPA{cli}

			cond, err := hpa.Check(ctx, tt.quay)
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
