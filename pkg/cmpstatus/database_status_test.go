package cmpstatus

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// componentChecker defines an interface for component status checkers.
type componentChecker interface {
	Check(context.Context, qv1.QuayRegistry) (qv1.Condition, error)
}

func newTestDeployment(quayName, deploymentSuffix, pvcName string, owned bool) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", quayName, deploymentSuffix),
			Namespace: "quay-namespace",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}
	if owned {
		dep.SetOwnerReferences([]metav1.OwnerReference{
			{
				Kind:       "QuayRegistry",
				Name:       quayName,
				APIVersion: "quay.redhat.com/v1",
				UID:        types.UID(quayName + "-uid"),
			},
		})
	}
	return dep
}

func newTestPVC(name string, phase corev1.PersistentVolumeClaimPhase) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "quay-namespace",
			UID:       types.UID(name + "-pvc-uid"),
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: phase,
		},
	}
}

func TestDatabaseComponentCheck(t *testing.T) {
	type testCase struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}

	componentTests := []struct {
		checker          func(client.Client) componentChecker
		componentKind    qv1.ComponentKind
		componentName    string
		deploymentSuffix string
		readyType        qv1.ConditionType
	}{
		{
			checker:          func(c client.Client) componentChecker { return &Postgres{Client: c} },
			componentKind:    qv1.ComponentPostgres,
			componentName:    "Postgres",
			deploymentSuffix: "quay-database",
			readyType:        qv1.ComponentPostgresReady,
		},
		{
			checker:          func(c client.Client) componentChecker { return &ClairPostgres{Client: c} },
			componentKind:    qv1.ComponentClairPostgres,
			componentName:    "ClairPostgres",
			deploymentSuffix: "clair-postgres",
			readyType:        qv1.ComponentClairPostgresReady,
		},
	}

	for _, ct := range componentTests {
		pvcName := fmt.Sprintf("%s-pvc", ct.deploymentSuffix)
		deploymentName := fmt.Sprintf("registry-%s", ct.deploymentSuffix)

		testCases := []testCase{
			{
				name: "unmanaged",
				quay: qv1.QuayRegistry{
					Spec: qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: false}}},
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentUnmanaged,
					Message: fmt.Sprintf("%s not managed by the operator", ct.componentName),
				},
			},
			{
				name: "deployment not found",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: fmt.Sprintf("%s deployment %s not found", ct.componentName, deploymentName),
				},
			},
			{
				name: "deployment not owned",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				objs: []client.Object{
					newTestDeployment("registry", ct.deploymentSuffix, pvcName, false),
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: fmt.Sprintf("%s deployment %s not owned by QuayRegistry", ct.componentName, deploymentName),
				},
			},
			{
				name: "pvc pending",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				objs: []client.Object{
					newTestDeployment("registry", ct.deploymentSuffix, pvcName, true),
					newTestPVC(pvcName, corev1.ClaimPending),
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonPVCPending,
					Message: fmt.Sprintf("%s PersistentVolumeClaim %s is pending", ct.componentName, pvcName),
				},
			},
			{
				name: "provisioning failed",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				objs: []client.Object{
					newTestDeployment("registry", ct.deploymentSuffix, pvcName, true),
					newTestPVC(pvcName, corev1.ClaimPending),
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{Name: "pvc-fail-event", Namespace: "quay-namespace"},
						InvolvedObject: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "PersistentVolumeClaim",
							Name:       pvcName,
							UID:        types.UID(pvcName + "-pvc-uid"),
						},
						Reason:  "ProvisioningFailed",
						Message: "storage class not found",
					},
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonPVCProvisioningFailed,
					Message: fmt.Sprintf("%s PVC %s provisioning failed: storage class not found", ct.componentName, pvcName),
				},
			},
			{
				name: "unhealthy deployment",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				objs: []client.Object{
					func() *appsv1.Deployment {
						dep := newTestDeployment("registry", ct.deploymentSuffix, pvcName, true)
						dep.Status.AvailableReplicas = 0
						return dep
					}(),
					newTestPVC(pvcName, corev1.ClaimBound),
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: fmt.Sprintf("Deployment %s has zero replicas available", deploymentName),
				},
			},
			{
				name: "all healthy",
				quay: qv1.QuayRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "quay-namespace", UID: "registry-uid"},
					Spec:       qv1.QuayRegistrySpec{Components: []qv1.Component{{Kind: ct.componentKind, Managed: true}}},
				},
				objs: []client.Object{
					func() *appsv1.Deployment {
						dep := newTestDeployment("registry", ct.deploymentSuffix, pvcName, true)
						dep.Status.AvailableReplicas = 1
						dep.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
						return dep
					}(),
					newTestPVC(pvcName, corev1.ClaimBound),
				},
				cond: qv1.Condition{
					Type:    ct.readyType,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: fmt.Sprintf("Deployment %s healthy", deploymentName),
				},
			},
		}

		for _, tt := range testCases {
			t.Run(fmt.Sprintf("%s/%s", ct.componentName, tt.name), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				scheme := runtime.NewScheme()
				qv1.AddToScheme(scheme)
				clientgoscheme.AddToScheme(scheme)

				cliBuilder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...)
				cliBuilder.WithIndex(&corev1.Event{}, "involvedObject.uid", func(rawObj client.Object) []string {
					event := rawObj.(*corev1.Event)
					return []string{string(event.InvolvedObject.UID)}
				})
				cli := cliBuilder.Build()

				checker := ct.checker(cli)
				cond, err := checker.Check(ctx, tt.quay)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}

				cond.LastUpdateTime = metav1.NewTime(time.Time{})
				if !reflect.DeepEqual(tt.cond, cond) {
					t.Errorf("expecting %+v, received %+v", tt.cond, cond)
				}
			})
		}
	}
}
