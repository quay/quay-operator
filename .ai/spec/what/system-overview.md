# System Overview

The quay-operator is a Kubernetes operator that deploys and manages [Quay container registry](https://github.com/quay/quay) instances on OpenShift and Kubernetes clusters. It reconciles `QuayRegistry` custom resources (API group `quay.redhat.com/v1`) into the set of Kubernetes objects needed to run Quay and its dependencies.

## Behavioral Rules

### Reconciliation Contract

1. The operator runs two independent controllers watching `QuayRegistry` resources. The main reconciler (`QuayRegistryReconciler`) handles the full lifecycle — creation, updates, version migration, and deletion. The status reconciler (`QuayRegistryStatusReconciler`) evaluates component health and updates status conditions.
2. The status reconciler runs on a separate 1-minute requeue loop. It is separated from the main reconciler because the main reconciler runs database migrations on version changes, which are expensive and must not re-run on every status check.
3. The main reconciler follows an 8-step sequence on each reconcile: (1) deletion handling, (2) migration/upgrade check, (3) config bundle creation, (4) context gathering, (5) component validation, (6) kustomize manifest inflation, (7) object application to cluster, (8) status update.
4. If any step fails, the reconciler sets a condition on the `QuayRegistry` status and requeues for retry.
5. The operator uses server-side apply with `ForceOwnership` for all managed resources. Any field on a managed resource that is not in the operator's applied manifest will be removed on the next reconcile. External patches to managed resources are overwritten.

### Version Migration

6. A version migration is triggered when `Status.CurrentVersion` differs from the `QUAY_VERSION` environment variable. This creates a `<name>-quay-app-upgrade` Job that runs Quay's alembic database migrations.
7. The upgrade Job must not be recreated if it has `Succeeded >= 1` or `Active >= 1`. Quay's migrations are not idempotent — re-running them can corrupt data.
8. While the migration job is active (`Active == 1`), the main reconciler short-circuits and requeues without proceeding to later steps.
9. If the migration job fails, the `ComponentsCreated` condition is set to `ConditionReasonMigrationsFailed` and the reconciler proceeds on the next pass (allowing Kubernetes to retry the job).
10. If the migration job is manually deleted while expected, the condition is set to `ConditionReasonMigrationsJobMissing` and the reconciler proceeds.

### PostgreSQL Major Version Upgrade

11. PostgreSQL major version upgrades are detected by comparing the deployed image repository name against the expected image. When a mismatch is found, the old deployment is scaled to zero, an upgrade Job runs, and on success the old deployment/service/PVC are cleaned up.
12. The old PVC is only deleted if the `POSTGRES_UPGRADE_DELETE_BACKUP` environment variable is explicitly set to a value other than `"false"`.

### Feature Detection

13. Cluster capabilities are detected at startup and cached for the operator's lifetime:
    - **Routes**: The Route API (`route.openshift.io/v1`) is probed by creating a temporary Route resource. The cluster hostname is extracted from the Route's `status.ingress[0].routerCanonicalHostname`. The wildcard TLS certificate is extracted via TLS dial to the Route's host on port 443.
    - **ObjectStorage**: The ObjectBucketClaim API (`objectbucket.io/v1alpha1`) is detected via `List` call.
    - **Monitoring**: The ServiceMonitor and PrometheusRule APIs (`monitoring.coreos.com/v1`) are detected via `List` call.
14. Components whose required API is not available are automatically set to `managed: false` when not explicitly declared by the user.

### Finalizer Lifecycle

15. The `quay-operator/finalizer` is added to every `QuayRegistry` on creation.
16. On deletion, the finalizer triggers cleanup of cluster-scoped resources (Grafana dashboard ConfigMap in `openshift-config-managed`, namespace monitoring label) before the `QuayRegistry` is removed.

### Requeue Behavior

17. The main reconciler's requeue interval is configured at startup via `--requeue-interval` (default 5 seconds in the CSV, configurable).
18. The status reconciler always requeues after 1 minute.

## Configuration Surface

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.configBundleSecret` | `string` | (auto-created) | Name of the Secret containing Quay's `config.yaml` and optional certs |
| `spec.components` | `[]Component` | All managed | List of components with `kind`, `managed`, `overrides`, and `secretRef` |
| `status.currentVersion` | `string` | — | The Quay version currently deployed |
| `status.registryEndpoint` | `string` | — | The external HTTPS URL for the registry |
| `status.conditions` | `[]Condition` | — | 14 condition types tracking component and overall health |

## Constraints

- The operator must maintain backward compatibility with the QuayRegistry v1 CRD across releases.
- The `QUAY_VERSION` environment variable must be set for version migration detection.
- Server-side apply with `ForceOwnership` means the operator is the sole owner of managed resource fields. Users must not directly patch managed resources — changes will be reverted.
