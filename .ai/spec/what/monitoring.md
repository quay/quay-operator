# Monitoring

The operator deploys Prometheus monitoring resources (ServiceMonitor, PrometheusRule) and optionally a Grafana dashboard for Quay metrics. Monitoring behavior depends on the operator's install mode.

## Behavioral Rules

### CRD Detection

1. Monitoring availability is checked by listing `ServiceMonitor` and `PrometheusRule` resources via the `monitoring.coreos.com/v1` API. If either CRD is not available, monitoring is disabled.
2. If monitoring CRDs are available, the operator checks for the `openshift-config-managed` namespace (required for the Grafana dashboard ConfigMap). If this namespace does not exist, monitoring is disabled.

### AllNamespaces Mode (Current)

3. Monitoring is only supported when the operator is running in AllNamespaces mode (`WatchNamespace` is empty). When `WatchNamespace` is set, monitoring is explicitly blocked with the message "monitoring is only supported in AllNamespaces mode".
4. The operator labels the QuayRegistry's namespace with `openshift.io/cluster-monitoring: "true"` so that the OpenShift cluster monitoring stack scrapes ServiceMonitors in that namespace.
5. A ServiceMonitor and PrometheusRule are created in the QuayRegistry's namespace via kustomize overlays.
6. A Grafana dashboard ConfigMap is created in the `openshift-config-managed` namespace. The dashboard JSON is customized with namespace and service filters matching the QuayRegistry instance.

### OwnNamespace Mode

7. [PLANNED: `docs/enhancements/monitoring-ownnamespace.md`] Monitoring in OwnNamespace mode will use User Workload Monitoring (UWM) instead of cluster monitoring. ServiceMonitor and PrometheusRule will be created in the watch namespace. Namespace labeling and Grafana dashboard will be skipped.

### Cleanup

8. On QuayRegistry deletion, the finalizer removes:
   - The `openshift.io/cluster-monitoring: "true"` label from the namespace
   - The Grafana dashboard ConfigMap from `openshift-config-managed`
9. Namespace-scoped monitoring resources (ServiceMonitor, PrometheusRule) are cleaned up by Kubernetes owner reference garbage collection.
10. When `WatchNamespace` is set, cleanup of cluster-scoped resources (namespace label, Grafana ConfigMap) is skipped entirely to avoid attempting operations that would fail with insufficient RBAC.

## Constraints

- The Grafana dashboard ConfigMap requires cross-namespace write access to `openshift-config-managed`. This is only available in AllNamespaces mode.
- Monitoring is tightly coupled to OpenShift's Prometheus stack. Vanilla Kubernetes clusters without Prometheus Operator CRDs cannot use managed monitoring.
