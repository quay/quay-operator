# Testing

## Unit Tests

```bash
# Run all tests with coverage
make test

# Run tests directly with verbose output
go test -v ./...

# Run a single test
go test -v -run TestFunctionName ./path/to/package/...

# Run tests for a specific package
go test -v ./pkg/kustomize/...
go test -v ./controllers/quay/...
```

## E2E Tests

E2E tests use [Chainsaw](https://kyverno.github.io/chainsaw/) v0.2.14 (Kubernetes test framework by Kyverno).

### Test Structure

Tests live in `test/chainsaw/` with each scenario in its own directory:

```text
test/chainsaw/
├── .chainsaw.yaml              # Shared config (timeouts, catch handlers, template: true)
├── Makefile                    # Chainsaw targets (included from root Makefile)
├── values-openshift.yaml       # Values for OpenShift (all components managed)
├── values-kind.yaml            # Values for KinD (route/objectstorage/monitoring/tls unmanaged)
├── reconcile/                  # Core reconcile lifecycle (create, mutate, delete, recover)
├── hpa/                        # HorizontalPodAutoscaler management
├── resource_overrides/         # Resource requests/limits and PVC volume overrides
├── ca_rotation/                # CA certificate rotation (destructive, OpenShift only)
├── custom_storageclass/        # Custom StorageClass for PVCs
├── unmanaged_clair/            # Clair unmanaged
├── unmanaged_postgres/         # External PostgreSQL
├── unmanaged_redis/            # External Redis
└── unmanaged_route_tls/        # Route/TLS combinations (OpenShift only)
```

### Running Tests

```bash
# OpenShift — all non-destructive tests
make test-e2e

# OpenShift — destructive tests (ca-rotation)
make test-e2e-destructive

# KinD — excludes OpenShift-only tests (ca-rotation, hpa, unmanaged-route)
hack/setup-kind-e2e.sh && make test-e2e-kind
```

Override parallelism and pass extra args:
```bash
CHAINSAW_PARALLEL=1 make test-e2e
CHAINSAW_EXTRA_ARGS="--skip-delete --assert-timeout 20m" make test-e2e-kind
```

### Environment Setup

#### KinD

```bash
# Creates KinD cluster, deploys Garage S3, stores creds in ConfigMap
hack/setup-kind-e2e.sh

# Build and run operator locally
make manager
SKIP_RESOURCE_REQUESTS=true ./bin/manager &

# Run tests
make test-e2e-kind
```

#### OpenShift (Prow CI)

The Prow CI pipeline handles setup automatically:
1. `quay-install-odf-operator` — installs ODF/NooBaa for managed object storage
2. `quay-install-operator-bundle` — installs operator via `operator-sdk run bundle`
3. `quay-test-chainsaw` — runs `make test-e2e` and `make test-e2e-destructive`

For manual testing on an OpenShift cluster with NooBaa already installed:
```bash
# Install operator via operator-sdk
operator-sdk run bundle --timeout=10m --security-context-config restricted \
  -n openshift-operators <BUNDLE_IMAGE>

# Run tests
KUBECONFIG=kubeconfig make test-e2e CHAINSAW_PARALLEL=1
```

### Values Files

Chainsaw templates use values from YAML files to control test behavior per platform:

- **`values-openshift.yaml`**: All components managed (route, objectstorage, monitoring, tls, hpa)
- **`values-kind.yaml`**: Route, objectstorage, monitoring, TLS, and HPA unmanaged (not available on KinD)

Values are accessed in test YAML via `($values.fieldName)`.

### Writing New Tests

Each test directory contains:
- `chainsaw-test.yaml` — test definition with ordered steps
- `NN-create-*.yaml` — resources to apply
- `NN-assert-*.yaml` — expected state assertions

#### Test structure

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: my-test
spec:
  steps:
  - name: step-name
    try:
    - apply:
        file: 00-create-quay-registry.yaml
    - assert:
        file: 00-assert-status.yaml
  - name: verify-something
    try:
    - script:
        shell: /bin/bash
        content: |
          set -euo pipefail
          # $NAMESPACE is set by chainsaw
          kubectl get deployment my-deploy -n $NAMESPACE -o jsonpath='{...}'
```

#### Platform-aware steps

Use OpenShift detection for platform-specific logic:
```bash
if kubectl api-resources 2>/dev/null | grep route.openshift.io >/dev/null; then
  echo "OpenShift detected"
else
  echo "KinD detected"
fi
```

#### Config bundle pattern (KinD)

KinD tests create a config bundle secret with platform-specific settings:
```bash
kubectl create secret generic my-config -n $NAMESPACE \
  --from-literal=config.yaml="${CONFIG}" \
  --from-file=ssl.cert=/tmp/ssl.cert \
  --from-file=ssl.key=/tmp/ssl.key
```

### CI Integration

| Job | Platform | What it does |
|-----|----------|-------------|
| GH Actions `e2e-kind.yaml` | KinD | Builds operator, runs `make test-e2e-kind` |
| Prow `e2e` | OpenShift | Installs via `operator-sdk run bundle`, runs chainsaw tests |
| Prow `e2e-upgrade` | OpenShift | Installs stable from catalog, upgrades to CI-built bundle, tests push/pull |

### Test Environment

- Unit tests use envtest (kubebuilder's test environment) — spins up local API server and etcd, no real cluster required
- E2E tests require a real cluster with `KUBECONFIG` set
- Chainsaw creates isolated namespaces per test run
