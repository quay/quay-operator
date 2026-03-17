package cmpstatus

import (
	"context"
	"time"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectStorage is capable of verifying if component ObjectStorage status. Inspects created
// ObjectBucketClaims and try to locate among them one that is owned by the QuayRegistry object,
// verifying at last its phase.
type ObjectStorage struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (o *ObjectStorage) Name() string {
	return "objectstorage"
}

// Check verifies if ObjectStorage component status is bound. Returns a quay Condition.
func (o *ObjectStorage) Check(
	ctx context.Context, reg qv1.QuayRegistry,
) (qv1.Condition, error) {
	var zero qv1.Condition

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentObjectStorage) {
		return qv1.Condition{
			Type:           qv1.ComponentObjectStorageReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Object storage not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	var list unstructured.UnstructuredList
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "objectbucket.io",
		Version: "v1alpha1",
		Kind:    "ObjectBucketClaimList",
	})
	if err := o.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return zero, err
	}

	for i := range list.Items {
		obc := &list.Items[i]
		if !qv1.Owns(reg, obc) {
			continue
		}

		phase, _, _ := unstructured.NestedString(obc.Object, "status", "phase")
		if phase != "Bound" {
			return qv1.Condition{
				Type:           qv1.ComponentObjectStorageReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Object bucket claim not bound",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentObjectStorageReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentReady,
			Message:        "Object bucket claim bound",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentObjectStorageReady,
		Status:         metav1.ConditionFalse,
		Reason:         qv1.ConditionReasonComponentNotReady,
		Message:        "Unable to locate object bucket claim",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
