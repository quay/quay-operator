# Architecture

## Controller Pattern

The operator uses controller-runtime with two reconcilers:

### QuayRegistryReconciler (`controllers/quay/quayregistry_controller.go`)

Main reconciler handling the QuayRegistry lifecycle:

1. **Deletion handling** - Process finalizers when QuayRegistry is deleted
2. **Migration checks** - Wait for upgrade jobs to complete before proceeding
3. **Config bundle creation** - Generate initial config secret if missing
4. **Context gathering** - Detect cluster capabilities (Routes, ObjectStorage, Monitoring)
5. **Component validation** - Validate overrides and component configuration
6. **Kustomize inflation** - Generate Kubernetes manifests from Kustomize bases
7. **Object application** - Apply all generated resources to cluster
8. **Status update** - Set conditions and registry endpoint

### QuayRegistryStatusReconciler (`controllers/quay/quayregistry_status_controller.go`)

Periodically evaluates component health and updates status conditions.

## Kustomize-Based Manifest Generation

`pkg/kustomize/kustomize.go` inflates QuayRegistry specs into deployable objects:

- Reads base manifests from `kustomize/base/`
- Applies component overlays from `kustomize/components/`
- Injects runtime values (secrets, hostnames, TLS certs)
- Returns slice of `client.Object` ready for cluster application

## Component Status Evaluation

`pkg/cmpstatus/` contains per-component health checkers:

```
pkg/cmpstatus/
├── evaluator.go      # Orchestrates all component checks
├── clair.go          # Clair deployment health
├── postgres.go       # PostgreSQL StatefulSet health
├── quay.go           # Quay deployment health
├── redis.go          # Redis deployment health
└── ...
```

Each checker implements:
```go
type Checker interface {
    Name() string
    Check(context.Context, qv1.QuayRegistry) (qv1.Condition, error)
}
```

## Runtime Context

`pkg/context/context.go` carries state through reconciliation:

```go
type QuayRegistryContext struct {
    SupportsRoutes       bool   // OpenShift Route API available
    SupportsObjectStorage bool  // ObjectBucketClaim API available
    SupportsMonitoring   bool   // Prometheus API available
    ClusterHostname      string // For generating registry endpoint
    TLSCert, TLSKey      []byte // TLS configuration
    // ... storage, database, Clair settings
}
```

## Feature Detection

`controllers/quay/features.go` detects cluster capabilities:

- **Routes**: Checks for `route.openshift.io/v1` API
- **ObjectStorage**: Checks for `objectbucket.io/v1alpha1` API
- **Monitoring**: Checks for `monitoring.coreos.com/v1` API

Components are automatically managed/unmanaged based on available APIs.

## Config Validation

`pkg/middleware/middleware.go` validates and transforms Quay configuration:

- Validates config against JSON schema
- Ensures required fields for managed components
- Merges user config with operator-generated values
