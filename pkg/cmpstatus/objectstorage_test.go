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

func newUnstructuredOBC(name, namespace, phase string, ownerRefs []metav1.OwnerReference) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "objectbucket.io",
		Version: "v1alpha1",
		Kind:    "ObjectBucketClaim",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if len(ownerRefs) > 0 {
		obj.SetOwnerReferences(ownerRefs)
	}
	if phase != "" {
		_ = unstructured.SetNestedField(obj.Object, phase, "status", "phase")
	}
	return obj
}

func TestObjectStorageCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "unmanaged object storage",
			quay: qv1.QuayRegistry{
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentObjectStorageReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Object storage not managed by the operator",
			},
		},
		{
			name: "object storage not found",
			quay: qv1.QuayRegistry{
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentObjectStorageReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Unable to locate object bucket claim",
			},
		},
		{
			name: "object storage not bound",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				newUnstructuredOBC("", "", "Failed", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentObjectStorageReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Object bucket claim not bound",
			},
		},
		{
			name: "object storage bound",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				newUnstructuredOBC("", "", "Bound", []metav1.OwnerReference{
					{
						Kind:       "QuayRegistry",
						Name:       "registry",
						APIVersion: "quay.redhat.com/v1",
						UID:        "uid",
					},
				}),
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentObjectStorageReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Object bucket claim bound",
			},
		},
		{
			name: "object storage not owned",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				newUnstructuredOBC("", "", "Bound", nil),
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentObjectStorageReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Unable to locate object bucket claim",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()
			builder := fake.NewClientBuilder()
			cli := builder.WithObjects(tt.objs...).WithScheme(scheme).Build()
			obs := ObjectStorage{cli}

			cond, err := obs.Check(ctx, tt.quay)
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
