package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestQuayCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "quay app upgrade job not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Job registry-quay-app-upgrade not found",
			},
		},
		{
			name: "quay app upgrade job not finished",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 0,
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Job registry-quay-app-upgrade not finished",
			},
		},
		{
			name: "quay app deployment not found",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-quay-app not found",
			},
		},
		{
			name: "unhealthy quay app deployment",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-quay-app: something went wrong",
			},
		},
		{
			name: "missing config editor deployment",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-quay-config-editor not found",
			},
		},
		{
			name: "faulty config editor deployment",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-config-editor",
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
				Type:    qv1.ComponentQuayReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-quay-config-editor: something went wrong",
			},
		},
		{
			name: "all deployments working",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-config-editor",
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
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentReady,
				Status:  metav1.ConditionTrue,
				Message: "Quay component healthy",
			},
		},
		{
			name: "not owned quay app deploy",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app",
					},
					Status: appsv1.DeploymentStatus{
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Deployment registry-quay-app not owned by QuayRegistry",
			},
		},
		{
			name: "base component being scaled down",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind: qv1.ComponentQuay,
							Overrides: &qv1.Override{
								Replicas: pointer.Int32(0),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 2,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Quay component is being scaled down",
			},
		},
		{
			name: "base component scaled down",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind: qv1.ComponentQuay,
							Overrides: &qv1.Override{
								Replicas: pointer.Int32(0),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 0,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-config-editor",
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
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentReady,
				Status:  metav1.ConditionTrue,
				Message: "Quay component healthy",
			},
		},
		{
			name: "zero available replicas quay app",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 0,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Deployment registry-quay-app has zero replicas available",
			},
		},
		{
			name: "zero available replicas for config editor",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
			},
			objs: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-app-upgrade",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				},
				&appsv1.Deployment{
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
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-config-editor",
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
						AvailableReplicas: 0,
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentQuayReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Deployment registry-quay-config-editor has zero replicas available",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			quay := Quay{
				Client: cli,
			}

			cond, err := quay.Check(ctx, tt.quay)
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
