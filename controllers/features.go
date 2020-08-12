package controllers

import (
	"context"

	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/quay/quay-operator/api/v1"
)

func (r *QuayRegistryReconciler) checkRoutesAvailable(quay *v1.QuayRegistry) (*v1.QuayRegistry, error) {
	var routes routev1.RouteList
	if err := r.Client.List(context.Background(), &routes); err == nil && len(routes.Items) > 0 {
		r.Log.Info("cluster supports `Routes` API")
		existingAnnotations := quay.GetAnnotations()
		if existingAnnotations == nil {
			existingAnnotations = map[string]string{}
		}
		existingAnnotations[v1.ClusterHostnameAnnotation] = routes.Items[0].Status.Ingress[0].RouterCanonicalHostname
		quay.SetAnnotations(existingAnnotations)
	}

	return quay, nil
}

func (r *QuayRegistryReconciler) checkObjectBucketClaimsAvailable(quay *v1.QuayRegistry) (*v1.QuayRegistry, error) {
	datastoreName := types.NamespacedName{Namespace: quay.GetNamespace(), Name: quay.GetName() + "-quay-datastore"}
	var objectBucketClaims objectbucket.ObjectBucketClaimList
	if err := r.Client.List(context.Background(), &objectBucketClaims); err == nil {
		r.Log.Info("cluster supports `ObjectBucketClaims` API")

		existingAnnotations := quay.GetAnnotations()
		if existingAnnotations == nil {
			existingAnnotations = map[string]string{}
		}
		existingAnnotations[v1.SupportsObjectStorage] = "true"

		for _, obc := range objectBucketClaims.Items {
			if obc.GetNamespace()+"/"+obc.GetName() == datastoreName.String() {
				var datastoreSecret corev1.Secret
				if err = r.Client.Get(context.Background(), datastoreName, &datastoreSecret); err != nil {
					r.Log.Error(err, "unable to retrieve Quay datastore `Secret`")
					return nil, err
				}

				var datastoreConfig corev1.ConfigMap
				if err = r.Client.Get(context.Background(), datastoreName, &datastoreConfig); err != nil {
					r.Log.Error(err, "unable to retrieve Quay datastore `ConfigMap`")
					return nil, err
				}

				r.Log.Info("found `ObjectBucketClaim` and credentials `Secret`, `ConfigMap`")

				existingAnnotations[v1.StorageBucketNameAnnotation] = string(datastoreConfig.Data["BUCKET_NAME"])
				existingAnnotations[v1.StorageHostname] = string(datastoreConfig.Data["BUCKET_HOSTNAME"])
				existingAnnotations[v1.StorageAccessKeyAnnotation] = string(datastoreSecret.Data["AWS_ACCESS_KEY_ID"])
				existingAnnotations[v1.StorageSecretKeyAnnotation] = string(datastoreSecret.Data["AWS_SECRET_ACCESS_KEY"])
			}
		}
		quay.SetAnnotations(existingAnnotations)
	}

	return quay, nil
}
