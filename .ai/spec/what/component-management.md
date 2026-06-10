# Component Management

The operator manages Quay's backing services as discrete components. Each component can be `managed` (operator handles its lifecycle) or unmanaged (user provides the service externally).

## Behavioral Rules

### Component Inventory

1. The operator supports 11 component kinds: `quay`, `postgres`, `clair`, `clairpostgres`, `redis`, `horizontalpodautoscaler`, `objectstorage`, `route`, `mirror`, `monitoring`, `tls`.
2. Required components are: `postgres`, `objectstorage`, `route`, `redis`, `tls`. These must be either managed by the operator or provided externally by the user.
3. Optional components are: `clair`, `clairpostgres`, `horizontalpodautoscaler`, `mirror`, `monitoring`. These default to managed but can be disabled.
4. The `quay` component is always managed. If a user sets `managed: false` on the quay component, the operator silently overrides it to `true`.
5. Components not explicitly declared in `spec.components` are automatically added with default managed state based on cluster capabilities.

### Feature-Gated Defaults

6. `route` and `tls` default to managed only if the Route API (`route.openshift.io/v1`) is available. On vanilla Kubernetes without the Route API, they default to unmanaged.
7. `objectstorage` defaults to managed only if the ObjectBucketClaim API (`objectbucket.io/v1alpha1`) is available.
8. `monitoring` defaults to managed only if the ServiceMonitor and PrometheusRule APIs (`monitoring.coreos.com/v1`) are available.
9. `tls` defaults to managed when the Route API is available AND no user-provided TLS cert/key pair exists in the config bundle secret.
10. If a user explicitly sets a feature-gated component to `managed: true` but the required API is not available, the operator returns a validation error and blocks reconciliation.

### Override Validation

11. Overrides can only be set on managed components. Setting overrides on an unmanaged component is a validation error (enforced by CEL on the CRD and by `ValidateOverrides()` in the controller).
12. The override validation matrix defines which override fields are valid for which components:

| Override | quay | clair | mirror | postgres | clairpostgres | redis |
|----------|------|-------|--------|----------|---------------|-------|
| `volumeSize` | - | Yes | - | Yes | Yes | - |
| `storageClassName` | - | Yes | - | Yes | Yes | - |
| `env` | Yes | Yes | Yes | Yes | Yes | Yes |
| `replicas` | Yes | Yes | Yes | - | - | - |
| `affinity` | Yes | Yes | Yes | - | - | - |
| `resources` | Yes | Yes | Yes | Yes | Yes | Yes |
| `securityContext` | Yes | - | Yes | - | - | - |
| `labels` | Yes | Yes | Yes | Yes | Yes | Yes |
| `annotations` | Yes | Yes | Yes | Yes | Yes | Yes |

13. Setting an override on a component that does not support it is a validation error.

### HPA and Replicas Interaction

14. When the `horizontalpodautoscaler` component is managed, users cannot override `replicas` on any component — except to set it to `0` (scale-down). HPA and manual replicas compete; the operator enforces mutual exclusion.
15. When HPA is managed, default replica count is 2 for quay, clair, and mirror components.

### Label Override Restrictions

16. The labels `quay-component`, `app`, and `quay-operator/quayregistry` are protected and cannot be overridden by the user. These are used internally for resource selection and owner tracking.

### External TLS Secret

17. When the `tls` component is unmanaged, users may set `secretRef` to reference a `kubernetes.io/tls` Secret containing `tls.crt` and `tls.key`.
18. `secretRef` cannot be set on a managed component (enforced by CEL on the CRD).
19. `secretRef.name` must not be empty (enforced by CEL on the CRD).
20. The `secretRef` TLS secret and `ssl.cert`/`ssl.key` in `configBundleSecret` are mutually exclusive. If both are present, the operator returns an error.
21. The operator auto-labels the referenced secret with `quay.redhat.com/tls-secret: "true"` so the cache informer picks it up for reactive watches. When the secret's content changes, the operator triggers a rolling restart of Quay pods.

### Unmanaged Component Configuration

22. When a component is unmanaged, the user must provide the necessary configuration in `configBundleSecret`:
    - `postgres`: `DB_URI` and optionally `DATABASE_SECRET_KEY`
    - `redis`: `BUILDLOGS_REDIS` (and optionally `USER_EVENTS_REDIS`)
    - `objectstorage`: `DISTRIBUTED_STORAGE_CONFIG`
    - `clair`: Security scanner configuration
    - `route`: `SERVER_HOSTNAME`
    - `tls`: TLS cert/key pair via `ssl.cert`/`ssl.key` in the bundle or via `secretRef`
23. Components that support user config even when managed: `route` (SERVER_HOSTNAME), `mirror` (REPO_MIRROR_*), `redis` (PULL_METRICS_REDIS and other Redis config keys). For these components, user-provided config is preserved alongside operator-generated values.

## Constraints

- Adding a new component requires changes in 5 places: ComponentKind constant, AllComponents slice, kustomize manifests, status checker, and kustomize inflation logic.
- The component list in AllComponents determines the order components appear in the spec after defaulting.
