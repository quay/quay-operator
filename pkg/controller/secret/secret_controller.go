package secret

import (
	"context"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_secret")

// Add creates a new Secret Controller and adds it to the Manager. The Manager
// will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecret{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add creates the Controller to watch for Quay configuration Secret changes.
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	// Create the Controller
	controllerName := "quay-config-secret-controller"
	controllerOptions := controller.Options{Reconciler: r}
	c, err := controller.New(controllerName, mgr, controllerOptions)
	if err != nil {
		return err
	}

	// sourceKind is used to specify only Secrets should be watched
	sourceKind := source.Kind{
		Type: &corev1.Secret{},
	}

	// sourceFilters excludes any secrets this controller is not interested in.
	sourceFilters := predicate.Funcs{

		// Ignore non-update events
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },

		// Only handle updates to the Quay configuration Secret after N minutes
		UpdateFunc: func(e event.UpdateEvent) bool {
			// TODO: This shouldn't be hard-coded.
			//       The file `resources.go` can produce this value, but it's
			//       currently hard-coded there and requires a QuayEcosystem{}
			//       instance to be called.
			return e.MetaNew.GetName() == "quay-enterprise-config-secret"
		},
	}

	// Watch for changes to Quay's configuration Secret
	eventHandler := handler.EnqueueRequestForObject{}
	err = c.Watch(&sourceKind, &eventHandler, &sourceFilters)
	if err != nil {
		return err
	}

	return nil
}

// Ensure that ReconcileSecret implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSecret{}

// ReconcileSecret reconciles a Secret object
type ReconcileSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile any changes made to the Quay Configuration Secret
func (r *ReconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logging.Log.Info("Detected Quay config.yaml changed. Reconciling.")

	instance := &corev1.Secret{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Restart the Quay Configuration Pod(s)
	labels := client.MatchingLabels{"quay-enterprise-component": constants.LabelComponentConfigValue}
	err = r.client.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(request.Namespace), labels)
	if err != nil {
		logging.Log.Error(err, "Unable to restart Quay Configuration pod(s).")
		return reconcile.Result{Requeue: true}, nil
	} else {
		logging.Log.Info("Triggered restart of Quay Configuration pod(s).")
	}

	// Restart the Quay Pod(s)
	labels = client.MatchingLabels{"quay-enterprise-component": constants.LabelComponentAppValue}
	err = r.client.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(request.Namespace), labels)
	if err != nil {
		logging.Log.Error(err, "Unable to restart Quay application pod(s).")
		return reconcile.Result{Requeue: true}, nil
	} else {
		logging.Log.Info("Triggered restart of Quay application pod(s).")
	}

	return reconcile.Result{}, nil
}
