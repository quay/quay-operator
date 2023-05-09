package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	asv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ocsv1a1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestEvaluate(t *testing.T) {
	for _, tt := range []struct {
		name  string
		objs  []client.Object
		quay  qv1.QuayRegistry
		conds []qv1.Condition
	}{
		{
			name: "no objects",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClairPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentHPA,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMirror,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
						},
					},
				},
			},
			conds: []qv1.Condition{
				{
					Type:    qv1.ComponentHPAReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Horizontal pod autoscaler not found",
				},
				{
					Type:    qv1.ComponentRouteReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Route not found",
				},
				{
					Type:    qv1.ComponentMonitoringReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "PrometheusRule registry-quay-prometheus-rules not found",
				},
				{
					Type:    qv1.ComponentPostgresReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Postgres deployment not found",
				},
				{
					Type:    qv1.ComponentObjectStorageReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Unable to locate object bucket claim",
				},
				{
					Type:    qv1.ComponentClairReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Deployment registry-clair-app not found",
				},
				{
					Type:    qv1.ComponentClairPostgresReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Deployment registry-clair-postgres not found",
				},
				{
					Type:    qv1.ComponentTLSReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Config bundle secret not populated",
				},
				{
					Type:    qv1.ComponentRedisReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Redis deployment not found",
				},
				{
					Type:   qv1.ComponentQuayReady,
					Status: metav1.ConditionFalse,
					Reason: qv1.ConditionReasonComponentNotReady,
					Message: "Awaiting for component postgres," +
						"objectstorage,clair,clairpostgres,tls,redis to become " +
						"available",
				},
				{
					Type:    qv1.ComponentMirrorReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Awaiting for component quay to become available",
				},
			},
		},
		{
			name: "quay unhealthy",
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
						{
							Kind:    qv1.ComponentPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClairPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMirror,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
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
				&monv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-metrics-monitor",
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
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-database",
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
				&ocsv1a1.ObjectBucketClaim{
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
					Status: ocsv1a1.ObjectBucketClaimStatus{
						Phase: ocsv1a1.ObjectBucketClaimStatusPhaseBound,
					},
				},
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
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
				},
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
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
					},
				},
			},
			conds: []qv1.Condition{
				{
					Type:    qv1.ComponentHPAReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Horizontal pod autoscaler found",
				},
				{
					Type:    qv1.ComponentRouteReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Route admitted",
				},
				{
					Type:    qv1.ComponentMonitoringReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ServiceMonitor and PrometheusRules created",
				},
				{
					Type:    qv1.ComponentPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-database healthy",
				},
				{
					Type:    qv1.ComponentObjectStorageReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Object bucket claim bound",
				},
				{
					Type:    qv1.ComponentClairReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Clair component healthy",
				},
				{
					Type:    qv1.ComponentClairPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ClairPostgres component healthy",
				},
				{
					Type:    qv1.ComponentTLSReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Using cluster wildcard certs",
				},
				{
					Type:    qv1.ComponentRedisReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-redis healthy",
				},
				{
					Type:    qv1.ComponentQuayReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Deployment registry-quay-app not found",
				},
				{
					Type:    qv1.ComponentMirrorReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Awaiting for component quay to become available",
				},
			},
		},
		{
			name: "quay healthy",
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
						{
							Kind:    qv1.ComponentPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClairPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMirror,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
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
				&monv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-metrics-monitor",
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
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-database",
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
				&ocsv1a1.ObjectBucketClaim{
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
					Status: ocsv1a1.ObjectBucketClaimStatus{
						Phase: ocsv1a1.ObjectBucketClaimStatusPhaseBound,
					},
				},
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
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
				},
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
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
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
			conds: []qv1.Condition{
				{
					Type:    qv1.ComponentHPAReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Horizontal pod autoscaler found",
				},
				{
					Type:    qv1.ComponentRouteReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Route admitted",
				},
				{
					Type:    qv1.ComponentMonitoringReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ServiceMonitor and PrometheusRules created",
				},
				{
					Type:    qv1.ComponentPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-database healthy",
				},
				{
					Type:    qv1.ComponentObjectStorageReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Object bucket claim bound",
				},
				{
					Type:    qv1.ComponentClairReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Clair component healthy",
				},
				{
					Type:    qv1.ComponentClairPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ClairPostgres component healthy",
				},
				{
					Type:    qv1.ComponentTLSReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Using cluster wildcard certs",
				},
				{
					Type:    qv1.ComponentRedisReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-redis healthy",
				},
				{
					Type:    qv1.ComponentQuayReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Quay component healthy",
				},
				{
					Type:    qv1.ComponentMirrorReady,
					Status:  metav1.ConditionFalse,
					Reason:  qv1.ConditionReasonComponentNotReady,
					Message: "Mirror deployment not found",
				},
			},
		},
		{
			name: "all healthy",
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
						{
							Kind:    qv1.ComponentPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClair,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentClairPostgres,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRedis,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentObjectStorage,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentRoute,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMirror,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentMonitoring,
							Managed: true,
						},
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&asv2.HorizontalPodAutoscaler{
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
				&monv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-prometheus-rules",
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
				&monv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-metrics-monitor",
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
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry-quay-database",
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
				&ocsv1a1.ObjectBucketClaim{
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
					Status: ocsv1a1.ObjectBucketClaimStatus{
						Phase: ocsv1a1.ObjectBucketClaimStatusPhaseBound,
					},
				},
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
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
				},
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
								Status:  corev1.ConditionTrue,
								Message: "all good",
							},
						},
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
				&appsv1.Deployment{
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
			conds: []qv1.Condition{
				{
					Type:    qv1.ComponentHPAReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Horizontal pod autoscaler found",
				},
				{
					Type:    qv1.ComponentRouteReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Route admitted",
				},
				{
					Type:    qv1.ComponentMonitoringReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ServiceMonitor and PrometheusRules created",
				},
				{
					Type:    qv1.ComponentPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-database healthy",
				},
				{
					Type:    qv1.ComponentObjectStorageReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Object bucket claim bound",
				},
				{
					Type:    qv1.ComponentClairReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Clair component healthy",
				},
				{
					Type:    qv1.ComponentClairPostgresReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "ClairPostgres component healthy",
				},
				{
					Type:    qv1.ComponentTLSReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Using cluster wildcard certs",
				},
				{
					Type:    qv1.ComponentRedisReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-redis healthy",
				},
				{
					Type:    qv1.ComponentQuayReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Quay component healthy",
				},
				{
					Type:    qv1.ComponentMirrorReady,
					Status:  metav1.ConditionTrue,
					Reason:  qv1.ConditionReasonComponentReady,
					Message: "Deployment registry-quay-mirror healthy",
				},
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
			if err := ocsv1a1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding ocs to scheme: %s", err)
			}
			if err := asv2.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding hpa to scheme: %s", err)
			}
			if err := appsv1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding apps to scheme: %s", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding core to scheme: %s", err)
			}
			if err := monv1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding monitoring to scheme: %s", err)
			}
			if err := batchv1.AddToScheme(scheme); err != nil {
				t.Fatalf("unexpected error adding batch to scheme: %s", err)
			}

			builder := fake.NewClientBuilder()
			cli := builder.WithObjects(tt.objs...).WithScheme(scheme).Build()
			conds, err := Evaluate(ctx, cli, tt.quay)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			// remove the last update time as it is not relevant for the test.
			for i := range conds {
				conds[i].LastUpdateTime = metav1.NewTime(time.Time{})
			}

			if !reflect.DeepEqual(conds, tt.conds) {
				t.Errorf("expected %+v, received %+v", tt.conds, conds)
			}
		})
	}
}
