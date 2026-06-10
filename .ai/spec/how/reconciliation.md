# Reconciliation

The operator uses two independent controllers, both watching `QuayRegistry` resources. They are separated to avoid running expensive database migrations on every status poll.

## Module Map

| File | Key Symbols | Responsibility |
|---|---|---|
| `controllers/quay/quayregistry_controller.go` | `QuayRegistryReconciler`, `Reconcile()`, `manageQuayDeletion()`, `createInitialBundleSecret()`, `createOrUpdateObject()` | Full lifecycle reconciliation |
| `controllers/quay/quayregistry_status_controller.go` | `QuayRegistryStatusReconciler`, `Reconcile()` | Periodic health evaluation |
| `controllers/quay/features.go` | `checkManagedKeys()`, `ensureRouteDiscovery()`, `checkObjectBucketClaimsAvailable()`, `checkMonitoringAvailable()`, `checkManagedTLS()`, `checkExternalTLSSecret()`, `checkClusterCAHash()`, `checkManagedDatabaseReady()` | Context gathering functions |
| `controllers/quay/tls.go` | `checkTLSSecurityProfile()` | TLS security profile from APIServer |
| `pkg/kustomize/kustomize.go` | `Inflate()` | Kustomize manifest generation |
| `pkg/middleware/middleware.go` | `Process()` | Post-kustomize object transforms |
| `pkg/context/context.go` | `QuayRegistryContext` | State bag flowing through reconciliation |

## Data Flow

### Main Reconciler — 8-Step Loop

The `Reconcile()` function processes steps sequentially. Each step can short-circuit with a requeue or error condition.

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. DELETION CHECK                                               │
│    FlaggedForDeletion? → manageQuayDeletion() → return          │
├─────────────────────────────────────────────────────────────────┤
│ 2. MIGRATION CHECK                                              │
│    PostgresUpgradeRunning? → checkPostgresUpgradeStatus()       │
│    MigrationsRunning? → checkMigrationStatus()                  │
│    Both short-circuit: requeue without proceeding               │
├─────────────────────────────────────────────────────────────────┤
│ 3. CONFIG BUNDLE                                                │
│    NeedsBundleSecret? → createInitialBundleSecret() → requeue  │
│    Otherwise: GetConfigBundleSecret()                           │
├─────────────────────────────────────────────────────────────────┤
│ 4. CONTEXT GATHERING                                            │
│    NewQuayRegistryContext() → populate with:                    │
│    - checkExternalTLSSecret()                                   │
│    - checkManagedTLS()                                          │
│    - ensureRouteDiscovery()                                     │
│    - parseServerHostname() / fillServerHostname()               │
│    - checkObjectBucketClaimsAvailable()                         │
│    - checkMonitoringAvailable()                                 │
│    - checkManagedKeys()                                         │
│    - checkManagedDatabaseReady()                                │
│    - checkBuildManagerAvailable()                               │
│    - checkClusterCAHash()                                       │
│    - checkTLSSecurityProfile()                                  │
│    - checkNeedsPostgresUpgradeForComponent() (both PG + Clair)  │
├─────────────────────────────────────────────────────────────────┤
│ 5. COMPONENT VALIDATION                                         │
│    EnsureDefaultComponents() — add missing components           │
│    ValidateOverrides() — check override compatibility           │
├─────────────────────────────────────────────────────────────────┤
│ 6. KUSTOMIZE INFLATION                                          │
│    Inflate(quay, context) → []client.Object                     │
│    Selects overlays based on managed components                 │
│    Injects runtime values (secrets, hostnames, TLS)             │
├─────────────────────────────────────────────────────────────────┤
│ 7. OBJECT APPLICATION                                           │
│    For each object:                                             │
│    - middleware.Process() — apply overrides, strip resources    │
│    - createOrUpdateObject() — server-side apply w/ ForceOwner  │
│    Also: Grafana dashboard, namespace labeling, old secret GC  │
├─────────────────────────────────────────────────────────────────┤
│ 8. STATUS UPDATE                                                │
│    EnsureRegistryEndpoint()                                     │
│    Set ComponentsCreated condition                              │
│    Update status with endpoint + conditions                     │
└─────────────────────────────────────────────────────────────────┘
```

### Status Reconciler

The status reconciler runs independently on a 1-minute requeue:

1. Fetch the `QuayRegistry` resource
2. Call `cmpstatus.Evaluate()` to get conditions for all components
3. Merge evaluated conditions into existing status conditions
4. Update status

### Kustomize Inflation Pipeline

```
kustomize/base/              Base manifests (deployments, services, configmaps)
       │
       ▼
kustomize/components/<name>/ Component overlays (one per managed component)
       │                     Selected based on spec.components[].managed
       ▼
kustomize/overlays/current/  Active overlay combining base + components
       │
       ▼
pkg/kustomize.Inflate()      Runtime injection:
       │                     - Config secret references
       │                     - Image overrides (RELATED_IMAGE_*)
       │                     - Hostnames, TLS certs
       │                     - Version labels
       ▼
[]client.Object              Ready for middleware processing + cluster apply
```

### Middleware Processing

`pkg/middleware.Process()` iterates over each inflated object and applies transforms that cannot be expressed in Kustomize:

1. **Config secret flattening** — Merges user config bundle with operator-generated config. Strips TLS certs and extra CA certs from the rendered secret.
2. **Resource override injection** — Applies `resources.requests` and `resources.limits` from component overrides.
3. **Environment variable injection** — Merges user-specified env vars into container specs.
4. **Label/annotation injection** — Adds user-specified labels and annotations (with protected label checks).
5. **Affinity injection** — Sets pod affinity/anti-affinity rules from overrides.
6. **Replica override** — Sets deployment replica count from overrides.
7. **Security context override** — Sets container security context from overrides.
8. **Resource request trimming** — When `SKIP_RESOURCE_REQUESTS` is set, removes all resource requests and limits from all containers.

### QuayRegistryContext

The context is a plain struct (no interface, no methods besides constructor) that accumulates cluster state during step 4. It is consumed by:
- Kustomize inflation (step 6) — to select overlays and inject runtime values
- Middleware processing (step 7) — to apply context-dependent transforms
- Status update (step 8) — to set the registry endpoint

## Key Abstractions

- **Server-side apply with ForceOwnership**: The `createOrUpdateObject()` helper uses `client.Apply` with `client.ForceOwnership`. This means the operator "wins" all field conflicts — it is the sole manager of managed resources.
- **Requeue pattern**: The main reconciler stores `ctrl.Result{RequeueAfter: interval}` in `r.Requeue` and returns it from any step that needs retry. This ensures consistent requeue behavior.

## Integration Points

| Consumer | Provider | Mechanism |
|---|---|---|
| Main reconciler | Config bundle secret | `r.Get()` on the Secret named in `spec.configBundleSecret` |
| Main reconciler | Cluster APIs | RESTMapper for Route/OBC/Monitoring detection; direct Get/List for specific resources |
| Main reconciler | Kustomize | `Inflate()` function call with QuayRegistry + context |
| Main reconciler | Middleware | `Process()` function call per inflated object |
| Status reconciler | cmpstatus | `Evaluate()` function call with QuayRegistry |
| Both reconcilers | Kubernetes API | controller-runtime client (cached reads, direct writes) |

## Implementation Notes

- The main reconciler creates a `DeepCopy()` of the QuayRegistry at the start of each reconcile and works on the copy. This avoids mutating the cached object.
- Route discovery creates and deletes a temporary Route resource. The cluster hostname is cached in an `atomic.Pointer` for the operator's lifetime — it does not re-probe on subsequent reconciles.
- The `WatchNamespace` field on the reconciler restricts the operator to a single namespace. When set, the controller only watches resources in that namespace and skips cluster-scoped operations.
