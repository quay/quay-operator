package controllers

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	qv1 "github.com/quay/quay-operator/apis/quay/v1"
	"github.com/quay/quay-operator/pkg/cmpstatus"
)

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

	conds, err := cmpstatus.Evaluate(ctx, q.Client, reg)
	if err != nil {
		log.Error(err, "error retrieving QuayRegistry component conditions")
		return reschedule, nil
	}

	// uses the list of updated conditions to overwrite the QuayRegistry conditions.
	q.overwriteConditions(conds, &reg)

	if err := q.Client.Status().Update(ctx, &reg); err != nil {
		if errors.IsConflict(err) {
			log.Info("skipping status reconcile due to conflict, will retry")
			return reschedule, nil
		}
		log.Error(err, "unexpected error updating component conditions")
		return reschedule, nil
	}

	log.Info("quay components conditions reconciled")
	return reschedule, nil
}

// overwriteConditions glues the provided conditions into a QuayRegistry's status.conditions
// slice.  QuayRegistry conditions are overwritten in place.
func (q *QuayRegistryStatusReconciler) overwriteConditions(
	conds []qv1.Condition, reg *qv1.QuayRegistry,
) {
	var faultySeen bool
	for _, cond := range conds {
		curCond := qv1.GetCondition(reg.Status.Conditions, cond.Type)

		// initially updates last transition time to be last update time but if the
		// condition remains the same since we last checked overwrites it with the
		// previous last transition time. LastTransitionTime shows the last time the
		// status has changed (transition from ok to not ok or vice versa).
		cond.LastTransitionTime = cond.LastUpdateTime
		if curCond != nil && curCond.Status == cond.Status {
			cond.LastTransitionTime = curCond.LastTransitionTime
		}

		reg.Status.Conditions = qv1.SetCondition(reg.Status.Conditions, cond)
		if cond.Reason == qv1.ConditionReasonComponentNotReady {
			faultySeen = true
		}
	}

	// sets the overall condition for the QuayRegistry.
	status := metav1.ConditionTrue
	message := "All components reporting as healthy"
	reason := qv1.ConditionReasonHealthChecksPassing
	if faultySeen {
		status = metav1.ConditionFalse
		message = "Some components are not ready"
		reason = qv1.ConditionReasonComponentNotReady
	}

	availCond := qv1.GetCondition(reg.Status.Conditions, qv1.ConditionTypeAvailable)
	transition := metav1.NewTime(time.Now())
	if availCond != nil && availCond.Status == status {
		transition = availCond.LastTransitionTime
	}

	cond := qv1.Condition{
		Type:               qv1.ConditionTypeAvailable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastUpdateTime:     metav1.NewTime(time.Now()),
		LastTransitionTime: transition,
	}
	reg.Status.Conditions = qv1.SetCondition(reg.Status.Conditions, cond)

	// we do a final pass through all conditions set in the quay registry and drop all of
	// those that are not relevant anymore (most likely used in previous operator releases
	// and not used anymore).
	qv1.RemoveUnusedConditions(reg)
}
