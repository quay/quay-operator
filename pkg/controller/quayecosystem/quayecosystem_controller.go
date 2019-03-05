package quayecosystem

import (
	"context"

	"k8s.io/client-go/tools/record"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"

	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	oneInt32 int32 = 1
)

// Add creates a new QuayEcosystem Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileQuayEcosystem{client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetRecorder("quayecosystem-recorder")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("quayecosystem-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource QuayEcosystem
	err = c.Watch(&source.Kind{Type: &copv1alpha1.QuayEcosystem{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner QuayEcosystem
	//	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	//		IsController: true,
	//		OwnerType:    &copv1alpha1.QuayEcosystem{},
	//	})
	//	if err != nil {
	//		return err
	//	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileQuayEcosystem{}

// ReconcileQuayEcosystem reconciles a QuayEcosystem object
type ReconcileQuayEcosystem struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a QuayEcosytem object and makes changes based on the state read
// and what is in the Quay.Spec
func (r *ReconcileQuayEcosystem) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logrus.Info("Reconciling QuayEcosystem")

	// Fetch the Quay instance
	quayEcosystem := &copv1alpha1.QuayEcosystem{}
	err := r.client.Get(context.TODO(), request.NamespacedName, quayEcosystem)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	err = r.setDefaults(quayEcosystem)
	if err != nil {
		return reconcile.Result{}, err
	}

	configuration := configuration.New(r.client, r.scheme, quayEcosystem)

	valid, err := configuration.Validate(quayEcosystem)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !valid {
		r.recorder.Event(quayEcosystem, "Warning", "QuayEcosystem Validation Failure", "Failed to validate QuayEcosystem Custom Resource")
		return reconcile.Result{}, nil
	}

	result, err := configuration.Reconcile()

	if err != nil {
		return reconcile.Result{}, err
	}

	if result != nil {
		return *result, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileQuayEcosystem) setDefaults(quayEcosystem *copv1alpha1.QuayEcosystem) error {

	changed := false

	if len(quayEcosystem.Spec.Quay.Image) == 0 {
		logrus.Info("Setting default Quay image: " + constants.QuayImage)
		changed = true
		quayEcosystem.Spec.Quay.Image = constants.QuayImage
	}

	if quayEcosystem.Spec.Quay.Replicas == nil {
		logrus.Info("Setting default Quay replicas: 1")
		changed = true
		quayEcosystem.Spec.Quay.Replicas = &oneInt32
	}

	if !quayEcosystem.Spec.Redis.Skip {

		if len(quayEcosystem.Spec.Redis.Image) == 0 {
			logrus.Info("Setting default Redis image: " + constants.RedisImage)
			changed = true
			quayEcosystem.Spec.Redis.Image = constants.RedisImage
		}

		if quayEcosystem.Spec.Redis.Replicas == nil {
			logrus.Info("Setting default Redis replicas: 1")
			changed = true
			quayEcosystem.Spec.Redis.Replicas = &oneInt32
		}

	}

	if changed {
		return r.client.Update(context.TODO(), quayEcosystem)
	}

	return nil
}
