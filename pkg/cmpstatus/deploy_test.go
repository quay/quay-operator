package cmpstatus

import (
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestDeploy_check(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cond   qv1.Condition
		deploy appsv1.Deployment
	}{
		{
			name: "no available condition",
			deploy: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "does-not-exist",
				},
			},
			cond: qv1.Condition{
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Available condition not found for does-not-exist",
			},
		},
		{
			name: "deployment not available",
			deploy: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentAvailable,
							Status:  corev1.ConditionFalse,
							Message: "random failure",
						},
					},
				},
			},
			cond: qv1.Condition{
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Deployment abc: random failure",
			},
		},
		{
			name: "deployment available",
			deploy: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
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
			cond: qv1.Condition{
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Deployment abc healthy",
			},
		},
		{
			name: "deployment available with extra conditions",
			deploy: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentProgressing,
							Status:  corev1.ConditionFalse,
							Message: "all good",
						},
						{
							Type:    appsv1.DeploymentAvailable,
							Status:  corev1.ConditionTrue,
							Message: "all good",
						},
					},
				},
			},
			cond: qv1.Condition{
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Deployment abc healthy",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dep := deploy{}
			cond := dep.check(tt.deploy)

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
