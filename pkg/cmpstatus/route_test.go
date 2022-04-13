package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	routev1 "github.com/openshift/api/route/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestRouteCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "managed but not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-route",
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Route not found",
			},
		},
		{
			name: "managed but not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Route not found",
			},
		},
		{
			name: "unmanaged",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Route is not managed by the operator",
			},
		},
		{
			name: "managed found without ingress",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
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
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Route found but no ingress",
			},
		},
		{
			name: "route not admitted",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   routev1.RouteAdmitted,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Route not fully admitted",
			},
		},
		{
			name: "Route admitted",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   routev1.RouteAdmitted,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Route admitted",
			},
		},
		{
			name: "route not admitted for one ingress",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "QuayRegistry",
								Name:       "registry",
								APIVersion: "quay.redhat.com/v1",
								UID:        "uid",
							},
						},
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   "unknown",
										Status: corev1.ConditionFalse,
									},
								},
							},
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   routev1.RouteAdmitted,
										Status: corev1.ConditionTrue,
									},
								},
							},
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   routev1.RouteAdmitted,
										Status: corev1.ConditionTrue,
									},
								},
							},
							{
								Conditions: []routev1.RouteIngressCondition{
									{
										Type:   routev1.RouteAdmitted,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentRouteReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Route not fully admitted",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()
			if err := routev1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding routes to scheme: %s", err)
			}

			builder := fake.NewClientBuilder()
			cli := builder.WithObjects(tt.objs...).WithScheme(scheme).Build()
			route := Route{cli}

			cond, err := route.Check(ctx, tt.quay)
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
