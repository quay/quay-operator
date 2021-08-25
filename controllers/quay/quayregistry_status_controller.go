package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/go-logr/logr"
	ocsv1a1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

const (
	// all objects that are deployed as part of any of our components must contain this
	// annotation. it is used when evaluating the conditions for all the components.
	quayComponentAnnotation = "quay-component"
)

// ConditionFetcher is a function capable of returning a map of conditions indexed by component
// name.
type ConditionFetcher func(context.Context, qv1.QuayRegistry) (map[string][]qv1.Condition, error)

// QuayRegistryStatusReconciler updates status for QuayRegistry components. This status Reconciler
// has to live in a different controller to avoid having to have a resync period in the main Quay
// reconciler as it generally takes longer and also runs a dabase migration everytime it is called.
type QuayRegistryStatusReconciler struct {
	Client client.Client
	Log    logr.Logger
	Mtx    *sync.Mutex
}

// NewQuayRegistryStatusReconciler returns a new QuayRegistryStatusController configured to use
// the provided client.
func NewQuayRegistryStatusReconciler(cli client.Client) *QuayRegistryStatusReconciler {
	return &QuayRegistryStatusReconciler{
		Client: cli,
		Log:    ctrl.Log.WithName("controllers").WithName("QuayRegistryStatus"),
	}
}

// SetupWithManager sets up provided manager as the source of events for this reconciler.
func (q *QuayRegistryStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&qv1.QuayRegistry{}).Complete(q)
}

// Reconcile is called for reconcile status for a given QuayRegistry components. This function
// always rescheduled the same event on return, that makes this to run from time to time.
func (q *QuayRegistryStatusReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	q.Mtx.Lock()
	defer q.Mtx.Unlock()

	log := q.Log.WithValues("quayregistrystatus", req.NamespacedName)
	reschedule := ctrl.Result{RequeueAfter: time.Minute}

	var reg qv1.QuayRegistry
	if err := q.Client.Get(ctx, req.NamespacedName, &reg); err != nil {
		if errors.IsNotFound(err) {
			// the QuayRegistry is no more, we can simply ignore it from now
			// on, no need for a reschedule.
			return ctrl.Result{}, nil
		}
		log.Error(err, "error getting QuayRegistry object")
		return reschedule, nil
	}

	allconds := map[string][]qv1.Condition{}
	for _, fn := range []ConditionFetcher{
		q.faultyDeploymentConditions,
		q.faultyRouteConditions,
		q.faultyObjectBucketClaimConditions,
		q.faultyJobConditions,
	} {
		conditions, err := fn(ctx, reg)
		if err != nil {
			log.Error(err, "error retrieving QuayRegistry component conditions")
			return reschedule, nil
		}
		for cname, conds := range conditions {
			if _, ok := allconds[cname]; ok {
				allconds[cname] = append(allconds[cname], conds...)
				continue
			}
			allconds[cname] = conds
		}
	}

	uc, err := qv1.MapToUnhealthyComponents(allconds)
	if err != nil {
		log.Error(err, "error creating component conditions")
		return reschedule, nil
	}

	if equality.Semantic.DeepEqual(reg.Status.UnhealthyComponents, uc) {
		log.Info("quay components conditions reconciled (no changes)")
		return reschedule, nil
	}

	reg.Status.UnhealthyComponents = uc
	if err := q.Client.Status().Update(ctx, &reg); err != nil {
		log.Error(err, "unexpected error updating component conditions")
		return reschedule, nil
	}

	log.Info("quay components conditions reconciled")
	return reschedule, nil
}

// faultyRouteConditions returns all conditions for Routes that were not admitted yet by any
// reason. Looks for RouteAdmitted condition among the list of Route current conditions. Converts
// RouteAdmitted conditions into quayv1 Conditions before return.
func (q *QuayRegistryStatusReconciler) faultyRouteConditions(
	ctx context.Context, reg qv1.QuayRegistry,
) (map[string][]qv1.Condition, error) {
	var list routev1.RouteList
	if err := q.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return nil, err
	}

	conds := map[string][]qv1.Condition{}
	for _, rt := range list.Items {
		component, ok := rt.Annotations[quayComponentAnnotation]
		if !qv1.Owns(reg, &rt) || !ok {
			continue
		}

		for _, ingress := range rt.Status.Ingress {
			cond, found := q.ingressAdmittedCondition(ingress.Conditions)
			if !found {
				continue
			}

			if cond.Status == corev1.ConditionTrue {
				continue
			}

			msg := fmt.Sprintf("Route %q not admitted: %s", rt.Name, cond.Message)
			var lastTransition metav1.Time
			if !cond.LastTransitionTime.IsZero() {
				lastTransition = *cond.LastTransitionTime
			}
			conds[component] = append(
				conds[component],
				qv1.Condition{
					Type:               qv1.ConditionType(cond.Type),
					Status:             metav1.ConditionStatus(cond.Status),
					Reason:             qv1.ConditionReason(cond.Reason),
					Message:            msg,
					LastTransitionTime: lastTransition,
				},
			)
		}
	}
	return conds, nil
}

// ingressAdmittedCondition looks and returns the Admitted condition among provided list of
// ingress conditions. Returns the condition and a flag indicating if it was found or not.
func (q *QuayRegistryStatusReconciler) ingressAdmittedCondition(
	conds []routev1.RouteIngressCondition,
) (routev1.RouteIngressCondition, bool) {
	for _, cond := range conds {
		if cond.Type != routev1.RouteAdmitted {
			continue
		}
		return cond, true
	}
	return routev1.RouteIngressCondition{}, false
}

// faultyDeploymentConditions returns the faulty conditions present in any of the component
// Deployments. Evaluate the Deployment status by its Available condition.
func (q *QuayRegistryStatusReconciler) faultyDeploymentConditions(
	ctx context.Context, reg qv1.QuayRegistry,
) (map[string][]qv1.Condition, error) {
	var list appsv1.DeploymentList
	if err := q.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return nil, err
	}

	conds := map[string][]qv1.Condition{}
	for _, dep := range list.Items {
		component, ok := dep.Annotations[quayComponentAnnotation]
		if !qv1.Owns(reg, &dep) || !ok {
			continue
		}

		cond, found := q.deployAvailableCondition(dep.Status.Conditions)
		if !found {
			continue
		}

		if cond.Status == corev1.ConditionTrue {
			continue
		}

		msg := fmt.Sprintf("Deployment %s: %s", dep.Name, cond.Message)
		conds[component] = append(
			conds[component],
			qv1.Condition{
				Type:               qv1.ConditionType(cond.Type),
				Status:             metav1.ConditionStatus(cond.Status),
				Reason:             qv1.ConditionReason(cond.Reason),
				Message:            msg,
				LastUpdateTime:     cond.LastUpdateTime,
				LastTransitionTime: cond.LastTransitionTime,
			},
		)
	}
	return conds, nil
}

// deployAvailableCondition filters the provided list of conditions and returns the Available
// condition if found. Returns a boolean indicating if the Available condition was found or not.
func (q *QuayRegistryStatusReconciler) deployAvailableCondition(
	conds []appsv1.DeploymentCondition,
) (appsv1.DeploymentCondition, bool) {
	for _, cond := range conds {
		if cond.Type != appsv1.DeploymentAvailable {
			continue
		}
		return cond, true
	}
	return appsv1.DeploymentCondition{}, false
}

// faultyObjectBucketClaimConditions evaluates the ObjectBucketClaim phase and returns a faulty
// condition if it is not set to ObjectBucketClaimStatusPhaseBound.
func (q *QuayRegistryStatusReconciler) faultyObjectBucketClaimConditions(
	ctx context.Context, reg qv1.QuayRegistry,
) (map[string][]qv1.Condition, error) {

	if !qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentObjectStorage) {
		return map[string][]qv1.Condition{}, nil
	}

	var list ocsv1a1.ObjectBucketClaimList
	if err := q.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return nil, err
	}

	conds := map[string][]qv1.Condition{}
	for _, obc := range list.Items {
		component, ok := obc.Annotations[quayComponentAnnotation]
		if !qv1.Owns(reg, &obc) || !ok {
			continue
		}

		phase := obc.Status.Phase
		if phase == ocsv1a1.ObjectBucketClaimStatusPhaseBound {
			continue
		}

		msg := fmt.Sprintf("ObjectBucketClaim %s reporing phase %q", obc.Name, phase)
		conds[component] = append(
			conds[component],
			qv1.Condition{
				Type:    "ObjectBucketClaimPhase",
				Status:  metav1.ConditionFalse,
				Reason:  "ObjectBucketClaimNotBound",
				Message: msg,
			},
		)
	}
	return conds, nil
}

// faultyJobConditions looks for Jobs owned by provided QuayRegistry and filters possible faulty
// conditions from it. Converts JobConditions into quayv1 Conditions before returning.
func (q *QuayRegistryStatusReconciler) faultyJobConditions(
	ctx context.Context, reg qv1.QuayRegistry,
) (map[string][]qv1.Condition, error) {
	var list batchv1.JobList
	if err := q.Client.List(ctx, &list, client.InNamespace(reg.Namespace)); err != nil {
		return nil, err
	}

	conds := map[string][]qv1.Condition{}
	for _, job := range list.Items {
		component, ok := job.Annotations[quayComponentAnnotation]
		if !qv1.Owns(reg, &job) || !ok {
			continue
		}

		cond, found := q.jobFailedCondition(job.Status.Conditions)
		if !found {
			continue
		}

		msg := fmt.Sprintf("Job %s: %s", job.Name, cond.Message)
		conds[component] = append(
			conds[component],
			qv1.Condition{
				Type:               qv1.ConditionType(cond.Type),
				Status:             metav1.ConditionStatus(cond.Status),
				Reason:             qv1.ConditionReason(cond.Reason),
				Message:            msg,
				LastTransitionTime: cond.LastTransitionTime,
			},
		)
	}
	return conds, nil
}

// jobFailedCondition filters the provided list of JobsCondition by any Failed condition. Returns
// a bool indicating if any failed condition was found in the list.
func (q *QuayRegistryStatusReconciler) jobFailedCondition(
	conds []batchv1.JobCondition,
) (batchv1.JobCondition, bool) {
	for _, cond := range conds {
		if cond.Type != batchv1.JobFailed {
			continue
		}
		return cond, true
	}
	return batchv1.JobCondition{}, false
}
