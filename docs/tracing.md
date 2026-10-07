# Tracing

The Quay Operator can export OpenTelemetry traces of its reconciles over OTLP/HTTP. Tracing is off unless an OTLP endpoint is configured.

## Enabling

Set one of these environment variables on the operator deployment:

- `OTEL_EXPORTER_OTLP_ENDPOINT`, for example `http://jaeger-collector:4318`;
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, for example `http://jaeger-collector:4318/v1/traces`.

The other standard `OTEL_EXPORTER_OTLP_*` variables (headers, TLS, timeout) also apply. The service name defaults to `quay-operator` and can be changed with `OTEL_SERVICE_NAME`.

## Sampling

All traces are sampled by default. Use the standard sampler variables to change this:

- CI and debugging: `OTEL_TRACES_SAMPLER=always_on` (100%).
- Routine deployments: `OTEL_TRACES_SAMPLER=parentbased_traceidratio` and `OTEL_TRACES_SAMPLER_ARG=0.1` (10%).

## Spans

| Span | Covers |
|------|--------|
| `QuayRegistryReconciler.Reconcile` | One reconcile of the main controller (root span). |
| `QuayRegistryStatusReconciler.Reconcile` | One reconcile of the status controller (root span). |
| `ensureRouteDiscovery` | Discovery of the cluster route hostname. |
| `checkObjectBucketClaimsAvailable` | Readiness of the managed object storage claim. |
| `checkManagedDatabaseReady` | Readiness of the managed Quay and Clair databases. |
| `kustomize.Inflate` | Rendering the QuayRegistry into Kubernetes objects. |
| `apply` | Creating or updating the rendered objects. |
| `cmpstatus.Check` | One component status check. |

## Attributes

Root spans:

| Key | Values |
|-----|--------|
| `quay.registry.namespace` | QuayRegistry namespace. |
| `quay.registry.name` | QuayRegistry name. |
| `quay.registry.uid` | QuayRegistry UID (absent when the object was not found). |
| `quay.registry.generation` | QuayRegistry `metadata.generation`. |
| `quay.reconcile.id` | controller-runtime reconcile ID. |
| `quay.reconcile.outcome` | `success` (no requeue), `requeue` (requeue requested), `error` (an error was returned or recorded), `not_found` (QuayRegistry deleted). |
| `quay.reconcile.requeue_after_ms` | Requested `RequeueAfter` in milliseconds, `0` when none. |
| `quay.reconcile.wait_reason` | Why the reconcile stopped or requeued; see below. |
| `quay.reconcile.condition_reason` | QuayRegistry condition reason, set with wait reason `rollout_blocked`. |

`quay.reconcile.wait_reason` values:

| Value | Meaning |
|-------|---------|
| `deleting` | QuayRegistry is being deleted. |
| `upgrade` | Postgres upgrade required or in progress. |
| `migration` | Database migration in progress. |
| `initial_bundle_secret` | Creating the initial config bundle secret. |
| `route_pending` | Route hostname probe still in progress. |
| `spec_defaults` | Writing component defaults to `spec.components`. |
| `sts_pending` | AWS STS credentials not provisioned yet. |
| `read_only` | Read-only lifecycle stopped the reconcile. |
| `pg_scale_down` | Waiting for Postgres to scale down. |
| `rollout_blocked` | A `RolloutBlocked` condition was set. |
| `immutable_resource` | An immutable object is still being deleted before it is recreated. |
| `status_update_failed` | Updating QuayRegistry status failed. |
| `obc_pending` | Managed object storage not initialized yet. |
| `db_pending` | Managed database(s) not ready yet. |
| `conflict` | Status update conflicted (status controller). |
| `steady_state` | Reconcile finished; periodic requeue. |

Phase spans:

| Key | Span | Values |
|-----|------|--------|
| `quay.apply.defer_workloads` | `apply` | `true` when workload objects are deferred. |
| `quay.component` | `cmpstatus.Check` | Component name, for example `postgres` or `quay`. |
