# Status Evaluation

The status evaluation subsystem periodically checks the health of all QuayRegistry components and updates the CR's status conditions. It runs in a dedicated controller to avoid triggering database migrations on every health check.

## Module Map

| File | Key Symbols | Responsibility |
|---|---|---|
| `pkg/cmpstatus/evaluator.go` | `Checker` interface, `Evaluate()` | Orchestrates all component checks with dependency ordering |
| `pkg/cmpstatus/quay.go` | `Quay` checker | Quay app deployment readiness |
| `pkg/cmpstatus/postgres.go` | `Postgres` checker | PostgreSQL deployment readiness |
| `pkg/cmpstatus/clair.go` | `Clair` checker | Clair deployment readiness |
| `pkg/cmpstatus/clairpostgres.go` | `ClairPostgres` checker | Clair PostgreSQL deployment readiness |
| `pkg/cmpstatus/redis.go` | `Redis` checker | Redis deployment readiness |
| `pkg/cmpstatus/hpa.go` | `HPA` checker | HorizontalPodAutoscaler existence and status |
| `pkg/cmpstatus/objectstorage.go` | `ObjectStorage` checker | ObjectBucketClaim readiness |
| `pkg/cmpstatus/route.go` | `Route` checker | Route admission and host status |
| `pkg/cmpstatus/mirror.go` | `Mirror` checker | Mirror deployment readiness |
| `pkg/cmpstatus/monitoring.go` | `Monitoring` checker | ServiceMonitor/PrometheusRule existence |
| `pkg/cmpstatus/tls.go` | `TLS` checker | TLS secret existence |
| `pkg/cmpstatus/deploy.go` | Shared deployment check helper | Common logic for checking Deployment readiness |
| `pkg/cmpstatus/database_status.go` | Shared database status helper | Common logic for checking database StatefulSet/Deployment readiness |

## Data Flow

```
QuayRegistryStatusReconciler.Reconcile()
    │
    ▼
cmpstatus.Evaluate(ctx, client, quayregistry)
    │
    ├── Phase 1: Independent components (no dependencies)
    │   ├── HPA.Check()        → ComponentHPAReady condition
    │   ├── Route.Check()      → ComponentRouteReady condition
    │   └── Monitoring.Check() → ComponentMonitoringReady condition
    │
    ├── Phase 2: Quay dependencies (failures block Quay)
    │   ├── Postgres.Check()       → ComponentPostgresReady condition
    │   ├── ObjectStorage.Check()  → ComponentObjectStorageReady condition
    │   ├── Clair.Check()          → ComponentClairReady condition
    │   ├── ClairPostgres.Check()  → ComponentClairPostgresReady condition
    │   ├── TLS.Check()            → ComponentTLSReady condition
    │   └── Redis.Check()          → ComponentRedisReady condition
    │
    ├── If ANY Phase 2 component failed:
    │   ├── ComponentQuayReady = False ("Awaiting for component X,Y to become available")
    │   └── ComponentMirrorReady = False (same message)
    │
    └── If ALL Phase 2 components healthy:
        ├── Quay.Check()   → ComponentQuayReady condition
        └── Mirror.Check() → ComponentMirrorReady condition
    │
    ▼
Available condition = True if ALL component conditions are True
```

## Key Abstractions

### Checker Interface

```go
type Checker interface {
    Name() string
    Check(context.Context, qv1.QuayRegistry) (qv1.Condition, error)
}
```

Each checker returns a single `Condition`. Unmanaged components return `Status: True` with `Reason: ComponentNotManaged`.

### Dependency Ordering

The two-phase evaluation ensures that Quay and Mirror are only checked when their dependencies are healthy. If Postgres is down, the Quay check is skipped and its condition explicitly states which dependency is failing.

This prevents misleading "QuayReady: False" conditions that would otherwise not explain the root cause.

### Condition Types

14 condition types are used:

| Condition | Category |
|---|---|
| `Available` | Overall health — True when all components are ready |
| `RolloutBlocked` | Set by main reconciler when config is invalid or APIs are missing |
| `ComponentsCreated` | Set by main reconciler after object application. Also tracks migration status. |
| `ComponentQuayReady` | Quay app deployment |
| `ComponentPostgresReady` | PostgreSQL |
| `ComponentClairReady` | Clair |
| `ComponentClairPostgresReady` | Clair PostgreSQL |
| `ComponentRedisReady` | Redis |
| `ComponentHPAReady` | HPA |
| `ComponentObjectStorageReady` | Object storage |
| `ComponentRouteReady` | Route |
| `ComponentMirrorReady` | Mirror |
| `ComponentMonitoringReady` | Monitoring |
| `ComponentTLSReady` | TLS |

### Abnormal-True Principle

Conditions follow the "abnormal-true" principle: they are most useful when highlighting broken states. A condition at `Status: True` means the component is healthy and generally unremarkable. `Status: False` with a descriptive `Reason` and `Message` is the actionable signal.

## Implementation Notes

- Each checker receives the full `QuayRegistry` object and uses it to determine if the component is managed. Unmanaged components short-circuit with `ComponentNotManaged` reason.
- Deployment-based checkers (Quay, Clair, Mirror, Postgres, Redis) use a shared helper in `deploy.go` that checks `AvailableReplicas > 0`.
- The `Available` condition is a synthetic roll-up: it is True only when every `Component*Ready` condition is True.
- The status reconciler always requeues after 1 minute regardless of current state. This means conditions are refreshed continuously even when no spec changes occur.
