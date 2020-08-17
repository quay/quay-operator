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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quayredhatcomv1 "github.com/quay/quay-operator/api/v1"
	v1 "github.com/quay/quay-operator/api/v1"
	"github.com/quay/quay-operator/pkg/kustomize"
)

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

	updatedQuay, err := v1.EnsureDefaultComponents(&quay)
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

	deploymentObjects, err := kustomize.Inflate(updatedQuay, &configBundle, &secretKeysBundle, log)
	if err != nil {
		log.Error(err, "could not inflate QuayRegistry into Kubernetes objects")
		return ctrl.Result{}, err
	}

	for _, obj := range deploymentObjects {
		objectMeta, _ := meta.Accessor(obj)
		groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()

		log.Info("creating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

		err = r.Client.Create(ctx, obj)
		if err != nil && errors.IsAlreadyExists(err) {
			log.Info("updating object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)
			err = r.Client.Update(ctx, obj)
		}
		if err != nil {
			log.Error(err, "failed to create/update object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)

			return ctrl.Result{}, err
		}

		log.Info("successfully created/updated object", "Name", objectMeta.GetName(), "GroupVersionKind", groupVersionKind)
	}

	log.Info("all objects created/updated successfully")

	return ctrl.Result{}, nil
}

func (r *QuayRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quayredhatcomv1.QuayRegistry{}).
		Complete(r)
}
