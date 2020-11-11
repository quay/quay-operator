/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	quayredhatcomv1 "github.com/quay/quay-operator/apis/quay/v1"
	v1 "github.com/quay/quay-operator/apis/quay/v1"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const upgradePollInterval = time.Second * 10
const upgradePollTimeout = time.Second * 600

// QuayRegistryReconciler reconciles a QuayRegistry object
type QuayRegistryReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quay.redhat.com,resources=quayregistries/status,verbs=get;update;patch

func (r *QuayRegistryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("quayregistry", req.NamespacedName)

	log.Info("begin reconcile")

	var quay v1.QuayRegistry
	if err := r.Client.Get(ctx, req.NamespacedName, &quay); err != nil {
		if errors.IsNotFound(err) {
			log.Info("`QuayRegistry` deleted")
		}
		log.Error(err, "unable to retrieve QuayRegistry")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	updatedQuay := quay.DeepCopy()

	if available := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeAvailable); available != nil && available.Reason == v1.ConditionReasonMigrationsInProgress {
		log.Info("migrations in progress, skipping reconcile")

		return ctrl.Result{}, nil
	}

	if !v1.CanUpgrade(quay.Status.CurrentVersion) {
		err := fmt.Errorf("cannot upgrade %s => %s", quay.Status.CurrentVersion, v1.QuayVersionCurrent)

		return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonUpgradeUnsupported, err.Error())
	}

	if quay.Spec.ConfigBundleSecret == "" {
		log.Info("`spec.configBundleSecret` is unset. Creating base `Secret`")

		baseConfigBundle, err := v1.EnsureOwnerReference(&quay, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: quay.GetName() + "-config-bundle-",
				Namespace:    quay.GetNamespace(),
			},
			Data: map[string][]byte{
				"config.yaml": encode(kustomize.BaseConfig()),
			},
		})
		if err != nil {
			msg := fmt.Sprintf("unable to add owner reference to base config bundle `Secret`: %s", err)

			return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
		}

		if err := r.Client.Create(ctx, baseConfigBundle); err != nil {
			msg := fmt.Sprintf("unable to create base config bundle `Secret`: %s", err)

			return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
		}

		objectMeta, _ := meta.Accessor(baseConfigBundle)
		updatedQuay.Spec.ConfigBundleSecret = objectMeta.GetName()
		if err := r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "unable to update `spec.configBundleSecret`")
			return ctrl.Result{}, nil
		}

		log.Info("successfully updated `spec.configBundleSecret`")
		return ctrl.Result{}, nil
	}

	var configBundle corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: quay.GetNamespace(), Name: quay.Spec.ConfigBundleSecret}, &configBundle); err != nil {
		msg := fmt.Sprintf("unable to retrieve referenced `configBundleSecret`: %s, error: %s", quay.Spec.ConfigBundleSecret, err)

		return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
	}

	log.Info("successfully retrieved referenced `configBundleSecret`", "configBundleSecret", configBundle.GetName(), "resourceVersion", configBundle.GetResourceVersion())

	var secretKeysBundle corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: quay.GetNamespace(), Name: kustomize.SecretKeySecretName(&quay)}, &secretKeysBundle); err != nil {
		if !errors.IsNotFound(err) {
			msg := fmt.Sprintf("unable to retrieve secret keys bundle: %s", err)

			return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
		}
	}

	updatedQuay, err := r.checkRoutesAvailable(updatedQuay.DeepCopy())
	if err != nil {
		msg := fmt.Sprintf("could not check for `Routes` API: %s", err)

		return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonRouteComponentDependencyError, msg)
	}

	updatedQuay, err = r.checkObjectBucketClaimsAvailable(updatedQuay.DeepCopy())
	if err != nil {
		msg := fmt.Sprintf("could not check for `ObjectBucketClaims` API: %s", err)
		if _, err = r.updateWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonObjectStorageComponentDependencyError, msg); err != nil {
			log.Error(err, "failed to update `conditions` of `QuayRegistry`")
		}

		return ctrl.Result{RequeueAfter: time.Millisecond * 1000}, nil
	}

	updatedQuay, err = v1.EnsureDefaultComponents(updatedQuay.DeepCopy())
	if err != nil {
		log.Error(err, "could not ensure default `spec.components`")

		return ctrl.Result{}, nil
	}

	if !v1.ComponentsMatch(quay.Spec.Components, updatedQuay.Spec.Components) {
		log.Info("updating QuayRegistry `spec.components` to include defaults")
		if err = r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `spec.components` to include defaults")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, nil
	}

	var userProvidedConfig map[string]interface{}
	err = yaml.Unmarshal(configBundle.Data["config.yaml"], &userProvidedConfig)
	if err != nil {
		updatedQuay, err = r.updateWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, err.Error())
		if err != nil {
			log.Error(err, "failed to update `conditions` of `QuayRegistry`")

			return ctrl.Result{}, nil
		}
	}

	updatedQuay.Status.Conditions = v1.RemoveCondition(updatedQuay.Status.Conditions, v1.ConditionTypeRolloutBlocked)

	for _, component := range updatedQuay.Spec.Components {
		contains, err := kustomize.ContainsComponentConfig(userProvidedConfig, component.Kind)
		if err != nil {
			updatedQuay, err = r.updateWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, err.Error())
			if err != nil {
				log.Error(err, "failed to update `conditions` of `QuayRegistry`")

				return ctrl.Result{}, nil
			}
		}

		if component.Managed && contains {
			msg := fmt.Sprintf("%s component marked as managed, but `configBundleSecret` contains required fields", component.Kind)

			updatedQuay, err = r.updateWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
			if err != nil {
				log.Error(err, "failed to update `conditions` of `QuayRegistry`")

				return ctrl.Result{}, nil
			}
		} else if !component.Managed && v1.RequiredComponent(component.Kind) && !contains {
			msg := fmt.Sprintf("required component `%s` marked as unmanaged, but `configBundleSecret` is missing necessary fields", component.Kind)

			updatedQuay, err = r.updateWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, msg)
			if err != nil {
				log.Error(err, "failed to update `conditions` of `QuayRegistry`")

				return ctrl.Result{}, nil
			}
		}
	}

	log.Info("inflating QuayRegistry into Kubernetes objects using Kustomize")
	deploymentObjects, err := kustomize.Inflate(updatedQuay, &configBundle, &secretKeysBundle, log)
	if err != nil {
		log.Error(err, "could not inflate QuayRegistry into Kubernetes objects")

		return ctrl.Result{}, nil
	}

	updatedQuay = stripObjectBucketClaimAnnotations(updatedQuay)

	for _, obj := range deploymentObjects {
		err = r.createOrUpdateObject(ctx, obj, quay)
		if err != nil {
			msg := fmt.Sprintf("all Kubernetes objects not created/updated successfully: %s", err)

			return r.reconcileWithCondition(&quay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonComponentCreationFailed, msg)
		}
	}

	updatedQuay.Status.ConfigEditorCredentialsSecret = configEditorCredentialsSecretFrom(deploymentObjects)
	updatedQuay, _ = v1.EnsureConfigEditorEndpoint(updatedQuay)

	if c := v1.GetCondition(updatedQuay.Status.Conditions, v1.ConditionTypeRolloutBlocked); c != nil && c.Status == metav1.ConditionTrue && c.Reason == v1.ConditionReasonConfigInvalid {
		return r.reconcileWithCondition(updatedQuay, v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonConfigInvalid, c.Message)
	}

	updatedQuay, err = r.updateWithCondition(updatedQuay, v1.ConditionTypeRolloutBlocked, metav1.ConditionFalse, v1.ConditionReasonComponentsCreationSuccess, "all objects created/updated successfully")
	if err != nil {
		log.Error(err, "failed to update `conditions` of `QuayRegistry`")

		return ctrl.Result{}, nil
	}

	if _, ok := updatedQuay.GetAnnotations()[v1.ObjectStorageInitializedAnnotation]; !ok && v1.ComponentIsManaged(updatedQuay.Spec.Components, "objectstorage") {
		r.Log.Info("requeuing to populate values for managed component: `objectstorage`")

		return ctrl.Result{Requeue: true}, nil
	}

	if updatedQuay.Status.CurrentVersion != v1.QuayVersionCurrent {
		updatedQuay, err = r.updateWithCondition(updatedQuay, v1.ConditionTypeAvailable, metav1.ConditionFalse, v1.ConditionReasonMigrationsInProgress, "running database migrations")
		if err != nil {
			log.Error(err, "failed to update `conditions` of `QuayRegistry`")

			return ctrl.Result{}, nil
		}

		go func(quayRegistry *v1.QuayRegistry) {
			err = wait.Poll(upgradePollInterval, upgradePollTimeout, func() (bool, error) {
				log.Info("checking Quay upgrade deployment readiness")

				var upgradeDeployment appsv1.Deployment
				err = r.Client.Get(ctx, types.NamespacedName{Name: quayRegistry.GetName() + "-quay-app-upgrade", Namespace: quayRegistry.GetNamespace()}, &upgradeDeployment)
				if err != nil {
					log.Error(err, "could not retrieve Quay upgrade deployment during upgrade")

					return false, err
				}

				if upgradeDeployment.Spec.Size() < 1 {
					log.Info("upgrade deployment scaled down, skipping check")

					return true, nil
				}

				if upgradeDeployment.Status.ReadyReplicas > 0 {
					log.Info("Quay upgrade complete, updating `status.currentVersion`")

					updatedQuay, _ := v1.EnsureRegistryEndpoint(updatedQuay)
					msg := "all registry component healthchecks passing"
					condition := v1.Condition{
						Type:               v1.ConditionTypeAvailable,
						Status:             metav1.ConditionTrue,
						Reason:             v1.ConditionReasonHealthChecksPassing,
						Message:            msg,
						LastUpdateTime:     metav1.Now(),
						LastTransitionTime: metav1.Now(),
					}
					updatedQuay.Status.Conditions = v1.SetCondition(updatedQuay.Status.Conditions, condition)
					updatedQuay.Status.CurrentVersion = v1.QuayVersionCurrent
					r.EventRecorder.Event(updatedQuay, corev1.EventTypeNormal, string(v1.ConditionReasonHealthChecksPassing), msg)

					if err = r.Client.Status().Update(ctx, updatedQuay); err != nil {
						log.Error(err, "could not update QuayRegistry status with current version")

						return true, err
					}

					updatedQuay.Spec.Components = v1.EnsureComponents(updatedQuay.Spec.Components)
					if err = r.Client.Update(ctx, updatedQuay); err != nil {
						log.Error(err, "could not update QuayRegistry spec to complete upgrade")

						return true, err
					}

					log.Info("successfully updated `status` after Quay upgrade")

					return true, nil
				}

				return false, nil
			})

			if err != nil {
				log.Error(err, "Quay upgrade deployment never reached ready phase")
			}
		}(updatedQuay.DeepCopy())
	}

	return ctrl.Result{}, nil
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}

func decode(bytes []byte) interface{} {
	var value interface{}
	_ = yaml.Unmarshal(bytes, &value)

	return value
}

func (r *QuayRegistryReconciler) createOrUpdateObject(ctx context.Context, obj k8sruntime.Object, quay v1.QuayRegistry) error {
	objectMeta, _ := meta.Accessor(obj)
	groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()

	shouldIgnoreError := func(e error) bool {
		// Jobs are immutable after creation, so ignore the error.
		jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}

		return groupVersionKind == jobGVK.String()
	}

	log := r.Log.WithValues("quayregistry", quay.GetNamespace())
	log.Info("creating/updating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

	obj, err := v1.EnsureOwnerReference(&quay, obj)
	if err != nil {
		log.Error(err, "could not ensure `ownerReferences` before creating object", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)
		return err
	}

	// managedFields cannot be set on a PATCH.
	objectMeta.SetManagedFields([]metav1.ManagedFieldsEntry{})

	opts := []client.PatchOption{}
	opts = append([]client.PatchOption{client.ForceOwnership, client.FieldOwner("quay-operator")}, opts...)
	err = r.Client.Patch(ctx, obj, client.Apply, opts...)

	if err != nil && !shouldIgnoreError(err) {
		log.Error(err, "failed to create/update object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

		return err
	}

	log.Info("finished creating/updating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

	return nil
}

func (r *QuayRegistryReconciler) updateWithCondition(q *v1.QuayRegistry, t v1.ConditionType, s metav1.ConditionStatus, reason v1.ConditionReason, msg string) (*v1.QuayRegistry, error) {
	updatedQuay := q.DeepCopy()

	condition := v1.Condition{
		Type:               t,
		Status:             s,
		Reason:             reason,
		Message:            msg,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	updatedQuay.Status.Conditions = v1.SetCondition(q.Status.Conditions, condition)
	updatedQuay.Status.LastUpdate = time.Now().UTC().String()

	eventType := corev1.EventTypeNormal
	if s == metav1.ConditionTrue {
		eventType = corev1.EventTypeWarning
	}

	// FIXME(alecmerdler): Need to pause here because race condition between updating `conditions` multiple times changes `resourceVersion`...
	time.Sleep(1000 * time.Millisecond)

	// Fetch first to ensure we have the right `resourceVersion` for updates.
	var currentQuay v1.QuayRegistry
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: q.GetNamespace(), Name: q.GetName()}, &currentQuay); err != nil {
		return nil, err
	}
	updatedQuay.SetResourceVersion(currentQuay.GetResourceVersion())

	if err := r.Client.Status().Update(context.Background(), updatedQuay); err != nil {
		return nil, err
	}
	r.EventRecorder.Event(updatedQuay, eventType, string(reason), msg)

	return updatedQuay, nil
}

// reconcileWithCondition sets the given condition on the `QuayRegistry` and returns a reconcile result.
func (r *QuayRegistryReconciler) reconcileWithCondition(q *v1.QuayRegistry, t v1.ConditionType, s metav1.ConditionStatus, reason v1.ConditionReason, msg string) (ctrl.Result, error) {
	_, err := r.updateWithCondition(q, t, s, reason, msg)

	return ctrl.Result{}, err
}

func (r *QuayRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// FIXME(alecmerdler): Can we do this in the `init()` function in `main.go`...?
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add OpenShift `Route` API to scheme")

		return err
	}
	// FIXME(alecmerdler): Can we do this in the `init()` function in `main.go`...?
	if err := objectbucket.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add `ObjectBucketClaim` API to scheme")

		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quayredhatcomv1.QuayRegistry{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		// TODO(alecmerdler): Add `.Owns()` for every resource type we manage...
		Complete(r)
}
