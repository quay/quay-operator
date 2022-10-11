package controllers

import (
	"context"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *QuayRegistryReconciler) cleanupNamespaceLabels(ctx context.Context, quay *v1.QuayRegistry) error {
	var ns corev1.Namespace
	err := r.Client.Get(ctx, types.NamespacedName{Name: quay.GetNamespace()}, &ns)

	if err != nil {
		return err
	}

	var quayRegistryList v1.QuayRegistryList
	listOps := client.ListOptions{
		Namespace: quay.GetNamespace(),
	}

	if err := r.Client.List(ctx, &quayRegistryList, &listOps); err != nil {
		return err
	}

	if ns.Labels != nil && ns.Labels[quayOperatorManagedLabelKey] != "" && len(quayRegistryList.Items) == 1 {
		updatedNs := ns.DeepCopy()
		labels := make(map[string]string)
		for k, v := range updatedNs.Labels {
			labels[k] = v
		}
		delete(labels, clusterMonitoringLabelKey)
		delete(labels, quayOperatorManagedLabelKey)
		updatedNs.Labels = labels

		patch := client.MergeFrom(&ns)
		err = r.Client.Patch(context.Background(), updatedNs, patch)
		return err
	}

	return nil
}

func (r *QuayRegistryReconciler) cleanupGrafanaConfigMap(ctx context.Context, quay *v1.QuayRegistry) error {
	var grafanaConfigMap corev1.ConfigMap
	grafanaConfigMapName := types.NamespacedName{
		Name:      quay.GetName() + "-" + grafanaDashboardConfigMapNameSuffix,
		Namespace: grafanaDashboardConfigNamespace}

	if err := r.Client.Get(ctx, grafanaConfigMapName, &grafanaConfigMap); err == nil || !errors.IsNotFound(err) {
		return r.Client.Delete(ctx, &grafanaConfigMap)
	}

	return nil
}

func (r *QuayRegistryReconciler) finalizeQuay(ctx context.Context, quay *v1.QuayRegistry) error {
	// NOTE: `controller-runtime` hangs rather than return "forbidden" error if insufficient RBAC permissions, so we use `WatchNamespace` to skip (https://github.com/kubernetes-sigs/controller-runtime/issues/550).
	if r.WatchNamespace != "" {
		r.Log.Info("not running in all-namespaces mode, skipping finalizer step: namespace label cleanup")
	} else {
		r.Log.Info("cleaning up namespace labels")

		if err := r.cleanupNamespaceLabels(ctx, quay); err != nil {
			return err
		}
		r.Log.Info("successfully cleaned up namespace labels")
	}

	// NOTE: `controller-runtime` hangs rather than return "forbidden" error if insufficient RBAC permissions, so we use `WatchNamespace` to skip (https://github.com/kubernetes-sigs/controller-runtime/issues/550).
	if r.WatchNamespace != "" {
		r.Log.Info("not running in all-namespaces mode, skipping finalizer step: Grafana `ConfigMap` cleanup")
	} else {
		r.Log.Info("cleaning up Grafana `ConfigMap`")
		if err := r.cleanupGrafanaConfigMap(ctx, quay); err != nil {
			return err
		}
		r.Log.Info("successfully cleaned up grafana config map")
	}

	return nil
}
