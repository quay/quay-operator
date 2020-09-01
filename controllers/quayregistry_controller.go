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
	"time"

	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quayredhatcomv1 "github.com/quay/quay-operator/api/v1"
	v1 "github.com/quay/quay-operator/api/v1"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const upgradePollInterval = time.Second * 10
const upgradePollTimeout = time.Second * 120

// QuayRegistryReconciler reconciles a QuayRegistry object
type QuayRegistryReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quay.redhat.com.quay.redhat.com,resources=quayregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quay.redhat.com.quay.redhat.com,resources=quayregistries/status,verbs=get;update;patch
// TODO(alecmerdler): Define needed RBAC permissions for all consumed API resources...

func (r *QuayRegistryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("quayregistry", req.NamespacedName)

	log.Info("begin reconcile")

	var quay v1.QuayRegistry
	if err := r.Client.Get(ctx, req.NamespacedName, &quay); err != nil {
		log.Error(err, "unable to retrieve QuayRegistry")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	updatedQuay := quay.DeepCopy()

	if quay.Spec.ConfigBundleSecret == "" {
		log.Info("`spec.configBundleSecret` is unset. Creating base `Secret`")

		baseConfigBundle := corev1.Secret{
			// FIXME(alecmerdler): Might need some labels on it...
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: quay.GetName() + "-config-bundle-",
				Namespace:    quay.GetNamespace(),
			},
			Data: kustomize.BaseConfigBundle(),
		}

		if err := r.Client.Create(ctx, &baseConfigBundle); err != nil {
			log.Error(err, "unable to create base config bundle `Secret`")
			return ctrl.Result{}, err
		}

		updatedQuay.Spec.ConfigBundleSecret = baseConfigBundle.GetName()
		if err := r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "unable to update `spec.configBundleSecret`")
			return ctrl.Result{}, err
		}

		log.Info("successfully updated `spec.configBundleSecret`")
		return ctrl.Result{}, nil
	}

	var configBundle corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: quay.GetNamespace(), Name: quay.Spec.ConfigBundleSecret}, &configBundle); err != nil {
		log.Error(err, "unable to retrieve referenced `configBundleSecret`", "configBundleSecret", quay.Spec.ConfigBundleSecret)
		return ctrl.Result{}, err
	}

	var secretKeysBundle corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: quay.GetNamespace(), Name: kustomize.SecretKeySecretName(&quay)}, &secretKeysBundle); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to retrieve secret keys bundle")
			return ctrl.Result{}, err
		}
	}

	log.Info("successfully retrieved referenced `configBundleSecret`", "configBundleSecret", configBundle.GetName(), "resourceVersion", configBundle.GetResourceVersion())

	updatedQuay, err := v1.EnsureDesiredVersion(&quay)
	if err != nil {
		log.Error(err, "could not ensure `spec.desiredVersion`")
		return ctrl.Result{}, err
	}

	if quay.Spec.DesiredVersion != updatedQuay.Spec.DesiredVersion {
		log.Info("updating QuayRegistry `spec.desiredVersion`")
		if err = r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `spec.desiredVersion`")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	updatedQuay, err = r.checkRoutesAvailable(updatedQuay.DeepCopy())
	if err != nil {
		log.Error(err, "could not check for Routes API")
		return ctrl.Result{}, err
	}

	updatedQuay, err = r.checkObjectBucketClaimsAvailable(updatedQuay.DeepCopy())
	if err != nil {
		log.Error(err, "could not check for `ObjectBucketClaims` API")
		return ctrl.Result{}, err
	}

	updatedQuay, err = v1.EnsureDefaultComponents(updatedQuay.DeepCopy())
	if err != nil {
		log.Error(err, "could not ensure default `spec.components`")
		return ctrl.Result{}, err
	}

	if !v1.ComponentsMatch(quay.Spec.Components, updatedQuay.Spec.Components) {
		log.Info("updating QuayRegistry `spec.components` to include defaults")
		if err = r.Client.Update(ctx, updatedQuay); err != nil {
			log.Error(err, "failed to update `spec.components` to include defaults")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	deploymentObjects, updatedQuay, err := kustomize.Inflate(updatedQuay, &configBundle, &secretKeysBundle, log)
	if err != nil {
		log.Error(err, "could not inflate QuayRegistry into Kubernetes objects")
		return ctrl.Result{}, err
	}

	for _, obj := range deploymentObjects {
		_ = r.createOrUpdateObject(ctx, obj, quay)
	}
	log.Info("all objects created/updated successfully")

	if quay.Status.LastUpdate == "" {
		updatedQuay.Status.LastUpdate = time.Now().UTC().String()

		if err = r.Client.Status().Update(ctx, updatedQuay); err != nil {
			r.Log.Error(err, "could not update QuayRegistry `status.lastUpdate` after (re)deployment")
			return ctrl.Result{}, err
		}
	}

	if updatedQuay.Spec.DesiredVersion != updatedQuay.Status.CurrentVersion {
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

					updatedQuay.Status.CurrentVersion = updatedQuay.Spec.DesiredVersion
					updatedQuay, _ := v1.EnsureRegistryEndpoint(updatedQuay)
					updatedQuay, _ = v1.EnsureConfigEditorEndpoint(updatedQuay)
					err = r.Client.Status().Update(ctx, updatedQuay)
					if err != nil {
						log.Error(err, "could not update QuayRegistry status with current version")
						return true, err
					}
				}

				return upgradeDeployment.Status.ReadyReplicas > 0, nil
			})
		}(updatedQuay.DeepCopy())
	}

	return ctrl.Result{}, nil
}

func (r *QuayRegistryReconciler) createOrUpdateObject(ctx context.Context, obj k8sruntime.Object, quay v1.QuayRegistry) error {
	objectMeta, _ := meta.Accessor(obj)
	groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()

	log := r.Log.WithValues("quayregistry", quay.GetNamespace())
	log.Info("creating/updating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

	// managedFields cannot be set on a PATCH.
	objectMeta.SetManagedFields([]metav1.ManagedFieldsEntry{})

	opts := []client.PatchOption{}
	opts = append([]client.PatchOption{client.ForceOwnership, client.FieldOwner("quay-operator")}, opts...)
	err := r.Client.Patch(ctx, obj, client.Apply, opts...)
	if err != nil {
		log.Error(err, "failed to create/update object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)
		return err
	}

	log.Info("finished creating/updating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)
	return nil
}

func (r *QuayRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add OpenShift `Route` API to scheme")
		return err
	}
	if err := objectbucket.AddToScheme(mgr.GetScheme()); err != nil {
		r.Log.Error(err, "Failed to add `ObjectBucketClaim` API to scheme")
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quayredhatcomv1.QuayRegistry{}).
		// TODO(alecmerdler): Add `.Owns()` for every resource type we manage...
		Complete(r)
}
