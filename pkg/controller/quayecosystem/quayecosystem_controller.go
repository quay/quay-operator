package quayecosystem

import (
	"context"
	"reflect"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
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
	return &ReconcileQuayEcosystem{client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetRecorder("quayecosystem-controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("quayecosystem-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource QuayEcosystem
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.QuayEcosystem{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileQuayEcosystem implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileQuayEcosystem{}

// ReconcileQuayEcosystem reconciles a QuayEcosystem object
type ReconcileQuayEcosystem struct {
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a QuayEcosystem object and makes changes based on the state read
// and what is in the QuayEcosystem.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileQuayEcosystem) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logging.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling QuayEcosystem")

	// Fetch the Quay instance
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
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
		logging.Log.Error(err, "Failed to set default values")
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

func (r *ReconcileQuayEcosystem) setDefaults(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) error {

	changed := false

	if len(quayEcosystem.Spec.Quay.Image) == 0 {
		logging.Log.Info("Setting default Quay image: " + constants.QuayImage)
		changed = true
		quayEcosystem.Spec.Quay.Image = constants.QuayImage
	}

	if quayEcosystem.Spec.Quay.Replicas == nil {
		logging.Log.Info("Setting default Quay replicas: 1")
		changed = true
		quayEcosystem.Spec.Quay.Replicas = &oneInt32
	}

	if !quayEcosystem.Spec.Redis.Skip {

		if len(quayEcosystem.Spec.Redis.Image) == 0 {
			logging.Log.Info("Setting default Redis image: " + constants.RedisImage)
			changed = true
			quayEcosystem.Spec.Redis.Image = constants.RedisImage
		}

		if quayEcosystem.Spec.Redis.Replicas == nil {
			logging.Log.Info("Setting default Redis replicas: 1")
			changed = true
			quayEcosystem.Spec.Redis.Replicas = &oneInt32
		}

	}

	if (redhatcopv1alpha1.Database{}) != quayEcosystem.Spec.Quay.Database {

		// Check if database type has been defined
		if len(quayEcosystem.Spec.Quay.Database.Type) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.Database.Type = redhatcopv1alpha1.DatabaseMySQL
		}

		if len(quayEcosystem.Spec.Quay.Database.Image) == 0 {
			changed = true
			switch quayEcosystem.Spec.Quay.Database.Type {
			case redhatcopv1alpha1.DatabaseMySQL:
				quayEcosystem.Spec.Quay.Database.Image = constants.MySQLImage
			case redhatcopv1alpha1.DatabasePostgresql:
				quayEcosystem.Spec.Quay.Database.Image = constants.PostgresqlImage
			}
		}

		if len(quayEcosystem.Spec.Quay.Database.VolumeSize) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.Database.VolumeSize = constants.QuayPVCSize
		}

		if len(quayEcosystem.Spec.Quay.Database.Memory) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.Database.Memory = constants.DatabaseMemory
		}

		if len(quayEcosystem.Spec.Quay.Database.CPU) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.Database.CPU = constants.DatabaseCPU
		}

	}

	if !reflect.DeepEqual(redhatcopv1alpha1.RegistryStorage{}, quayEcosystem.Spec.Quay.RegistryStorage) {

		if len(quayEcosystem.Spec.Quay.RegistryStorage.StorageDirectory) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.RegistryStorage.StorageDirectory = constants.QuayRegistryStorageDirectory
		}

		if len(quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.AccessModes) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.AccessModes = constants.QuayRegistryStoragePersistentVolumeAccessModes
		}

		if len(quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.Capacity) == 0 {
			changed = true
			quayEcosystem.Spec.Quay.RegistryStorage.PersistentVolume.Capacity = constants.QuayRegistryStoragePersistentVolumeStoreSize
		}

	}

	if changed {
		return r.client.Update(context.TODO(), quayEcosystem)
	}

	return nil
}
