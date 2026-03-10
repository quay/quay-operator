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

E2E tests use [kuttl](https://kuttl.dev/) (Kubernetes Test TooL).

```bash
# Run e2e tests (downloads kuttl if needed)
make test-e2e
```

### E2E Test Structure

Tests are in `e2e/` with each scenario in its own directory:

```
e2e/
├── happy_path/                    # Basic QuayRegistry creation
│   ├── 00-create-quay-registry.yaml
│   └── 00-assert.yaml
├── hpa/                           # HPA management scenarios
│   ├── 00-create-quay-registry.yaml
│   ├── 00-assert.yaml
│   ├── 01-unmanage-hpa.yaml
│   └── 01-assert.yaml
├── affinity_override/             # Affinity override tests
├── storageclass_overrides/        # StorageClass override tests
└── ...
```

### Kuttl Test Files

- `NN-*.yaml` - Test steps (create/update resources)
- `NN-assert.yaml` - Assertions (expected state)
- `NN-errors.yaml` - Expected errors (optional)

Steps execute in order (00, 01, 02...). Each step waits for its assertion to pass.

### Writing New E2E Tests

1. Create directory in `e2e/` for your scenario
2. Add `00-create-quay-registry.yaml` with initial QuayRegistry
3. Add `00-assert.yaml` with expected conditions/resources
4. Add numbered steps for mutations and their assertions

Example assertion (`00-assert.yaml`):
```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: test
status:
  conditions:
    - type: Available
      status: "True"
```

## Test Environment

Tests use envtest (kubebuilder's test environment):

- Spins up local API server and etcd
- No real cluster required for unit tests
- E2E tests require a real cluster with `KUBECONFIG` set
