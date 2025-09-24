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
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestClairCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "unmanaged clair",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentClairReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentUnmanaged,
				Message: "Clair not managed by the operator",
			},
		},
		{
			name: "missing clair deployment",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-clair-postgres",
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
				Type:    qv1.ComponentClairReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-clair-app not found",
			},
		},
		{
			name: "faulty clair deployment",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
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
				Type:    qv1.ComponentClairReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment registry-clair-app: something went wrong",
			},
		},
		{
			name: "all deployments working",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
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
				Type:    qv1.ComponentClairReady,
				Reason:  qv1.ConditionReasonComponentReady,
				Status:  metav1.ConditionTrue,
				Message: "Clair component healthy",
			},
		},
		{
			name: "not owned quay app deploy",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-clair-app",
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
				Type:    qv1.ComponentClairReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Deployment registry-clair-app not owned by QuayRegistry",
			},
		},
		{
			name: "zero replicas avail",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
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
				Type:    qv1.ComponentClairReady,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Status:  metav1.ConditionFalse,
				Message: "Deployment registry-clair-app has zero replicas available",
			},
		},
		{
			name: "scaled down",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
							Overrides: &qv1.Override{
								Replicas: ptr.To[int32](0),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
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
				Type:    qv1.ComponentClairReady,
				Reason:  qv1.ConditionReasonComponentReady,
				Status:  metav1.ConditionTrue,
				Message: "Clair manually scaled down",
			},
		},
		{
			name: "clair ephemeral volume PVC pending",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
							Overrides: &qv1.Override{
								UseEphemeralVolume: ptr.To(true),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-clair-app",
						OwnerReferences: []metav1.OwnerReference{{
							Kind:       "QuayRegistry",
							Name:       "registry",
							APIVersion: "quay.redhat.com/v1",
							UID:        "uid",
						}},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{{
									Name: "clair-tmp",
									VolumeSource: corev1.VolumeSource{
										Ephemeral: &corev1.EphemeralVolumeSource{
											VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{},
										},
									},
								}},
							},
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "rs1",
						Namespace:       "",
						OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "registry-clair-app"}},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs1"}},
					},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pvc1",
						OwnerReferences: []metav1.OwnerReference{{Kind: "Pod", Name: "pod1"}},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentClairReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonPVCPending,
				Message: "Clair /tmp PersistentVolumeClaim pvc1 is pending",
			},
		},
		{
			name: "clair ephemeral volume PVC provisioning failed",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
							Overrides: &qv1.Override{
								UseEphemeralVolume: ptr.To(true),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-clair-app",
						OwnerReferences: []metav1.OwnerReference{{
							Kind:       "QuayRegistry",
							Name:       "registry",
							APIVersion: "quay.redhat.com/v1",
							UID:        "uid",
						}},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{{
									Name: "clair-tmp",
									VolumeSource: corev1.VolumeSource{
										Ephemeral: &corev1.EphemeralVolumeSource{
											VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{},
										},
									},
								}},
							},
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "rs1",
						Namespace:       "",
						OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "registry-clair-app"}},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs1"}},
					},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pvc1",
						UID:             "pvcuid",
						OwnerReferences: []metav1.OwnerReference{{Kind: "Pod", Name: "pod1"}},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event1",
						Namespace: "",
					},
					InvolvedObject: corev1.ObjectReference{
						UID: "pvcuid",
					},
					Reason:  "ProvisioningFailed",
					Message: "provisioning failed for some reason",
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentClairReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonPVCProvisioningFailed,
				Message: "Clair /tmp PersistentVolumeClaim provisioning failed: provisioning failed for some reason",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()
			if err := qv1.AddToScheme(scheme); err != nil {
				t.Fatalf("failed to add quay/v1 to scheme: %s", err)
			}
			if err := clientgoscheme.AddToScheme(scheme); err != nil {
				t.Fatalf("failed to add client-go to scheme: %s", err)
			}

			cliBuilder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...)
			cliBuilder.WithIndex(&corev1.Event{}, "involvedObject.uid", func(rawObj client.Object) []string {
				event := rawObj.(*corev1.Event)
				return []string{string(event.InvolvedObject.UID)}
			})
			cli := cliBuilder.Build()
			clair := Clair{
				Client: cli,
			}

			cond, err := clair.Check(ctx, tt.quay)
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
