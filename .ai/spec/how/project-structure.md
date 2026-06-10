# Project Structure

## Module Map

| File/Directory | Key Symbols | Responsibility |
|---|---|---|
| `main.go` | `main()` | Entrypoint. Registers controllers, sets up health/readyz on `:8081`, metrics on `:8080`. |
| `apis/quay/v1/` | `QuayRegistry`, `QuayRegistrySpec`, `QuayRegistryStatus`, `Component`, `Override`, `ComponentKind` | CRD type definitions, validation helpers (`ValidateOverrides`, `EnsureDefaultComponents`), condition management (`SetCondition`, `GetCondition`). |
| `controllers/quay/quayregistry_controller.go` | `QuayRegistryReconciler`, `Reconcile()` | Main reconciliation loop. Handles the 8-step lifecycle from deletion check through object application. |
| `controllers/quay/quayregistry_status_controller.go` | `QuayRegistryStatusReconciler` | Periodic health evaluation on 1-minute requeue. Calls `cmpstatus.Evaluate()`. |
| `controllers/quay/features.go` | `checkManagedKeys()`, `checkObjectBucketClaimsAvailable()`, `checkMonitoringAvailable()`, `checkManagedTLS()`, `checkExternalTLSSecret()`, `ensureRouteDiscovery()`, `checkManagedDatabaseReady()`, `checkClusterCAHash()` | Context gathering — populates `QuayRegistryContext` with cluster state. |
| `controllers/quay/tls.go` | `checkTLSSecurityProfile()`, `translateTLSProfile()` | OpenShift TLS security profile inheritance from `APIServer` resource. |
| `pkg/kustomize/kustomize.go` | `Inflate()` | Manifest generation. Reads kustomize bases/overlays, injects runtime values, returns `[]client.Object`. |
| `pkg/kustomize/secrets.go` | Secret generation helpers | Config secret creation, managed keys secret, content hashing for deterministic output. |
| `pkg/cmpstatus/evaluator.go` | `Checker` interface, `Evaluate()` | Orchestrates all per-component health checks with dependency ordering. |
| `pkg/cmpstatus/<component>.go` | Per-component `Checker` implementations | Each file implements `Name()` + `Check()` for one component (clair, postgres, quay, redis, etc.). |
| `pkg/context/context.go` | `QuayRegistryContext` | Runtime state bag. Carries cluster capabilities, TLS config, storage creds, database state through reconciliation. |
| `pkg/middleware/middleware.go` | `Process()`, `FlattenSecret()` | Post-kustomize transforms. Applies overrides (env, replicas, resources, affinity, labels, annotations, security context), strips resource requests when `SKIP_RESOURCE_REQUESTS` is set, flattens config secrets. |
| `pkg/tls/validate.go` | `FetchAndValidate()` | Reads and validates TLS key pair from a Kubernetes Secret. |
| `kustomize/base/` | YAML manifests | Base Kubernetes resources shared across all configurations. |
| `kustomize/components/<name>/` | YAML overlays | Per-component kustomize overlays. Added when a component is managed. |
| `kustomize/overlays/current/` | Kustomization | The active overlay combining base + selected components. |
| `kustomize/app/` | Kustomization | Application-level overlay for the Quay deployment. |

## Key Entry Points

- **Operator startup**: `main.go` → registers both controllers with controller-runtime manager
- **Reconcile trigger**: Any change to a `QuayRegistry` resource or its owned resources → `QuayRegistryReconciler.Reconcile()`
- **Status evaluation**: 1-minute timer → `QuayRegistryStatusReconciler.Reconcile()` → `cmpstatus.Evaluate()`
- **Health probes**: `:8081/healthz` and `:8081/readyz`

## Naming Conventions

- Component kind names use lowercase concatenated words: `clairpostgres`, `objectstorage`, `horizontalpodautoscaler`
- Kubernetes resource names are prefixed with the QuayRegistry name: `<registry-name>-quay-app`, `<registry-name>-quay-database`, `<registry-name>-clair-postgres`
- Config secrets use the pattern: `<registry-name>-quay-config-secret-<hash>`
- Managed keys secret: `<registry-name>-quay-registry-managed-secret-keys`

## Environment Variables

| Variable | Purpose |
|---|---|
| `QUAY_VERSION` | Current operator version. Compared against `Status.CurrentVersion` to trigger migrations. |
| `SKIP_RESOURCE_REQUESTS` | When set, middleware strips all resource requests/limits from generated manifests. Useful for dev/test. |
| `RELATED_IMAGE_COMPONENT_QUAY` | Override the Quay container image. Similar vars exist for CLAIR, POSTGRES, REDIS. |
| `POSTGRES_UPGRADE_DELETE_BACKUP` | When set to a value other than `"false"`, old PostgreSQL PVCs are deleted after major version upgrade. |
| `WATCH_NAMESPACE` | Restricts the operator to a single namespace (OwnNamespace mode). Empty = AllNamespaces. |
