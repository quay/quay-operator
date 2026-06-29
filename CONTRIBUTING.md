# Contributing to quay-operator

## Setup

```bash
# Requires OpenShift/K8s cluster
make install   # Install CRDs
make deploy    # Deploy operator

# Local development
make run       # Run against configured cluster
```

## Development

Operator for deploying Quay on Kubernetes/OpenShift. Most code changes should start in one of these areas:

- API changes: `apis/quay/v1/`
- Reconcile changes: `controllers/quay/`
- Rendered Kubernetes resources: `pkg/kustomize/` and `kustomize/`
- Component readiness: `pkg/cmpstatus/`
- Cluster tests: `test/chainsaw/`

## Testing

```bash
# Unit tests
make test

# E2E tests (requires cluster)
make chainsaw-test

# Scorecard
operator-sdk scorecard bundle
```

## Making Changes

### CRD Changes
1. Update `apis/quay/v1/*_types.go`
2. Run `make generate` to update generated code
3. Run `make manifests` to regenerate CRDs, RBAC, webhooks, and bundle CRD output
4. Test upgrade path from previous version

### Controller Changes
- Update `controllers/quay/`
- Add tests in `controllers/*_test.go`
- Verify reconciliation doesn't fight with manual edits

### Manifest Changes
- Update base manifests under `kustomize/` or Kubebuilder manifests under `config/`
- Run `make manifests` when generated CRD/RBAC/webhook output is affected
- Verify bundle output under `bundle/` when operator packaging changes

## Pull Requests

- Test on OpenShift (primary) and vanilla K8s
- Document new QuayRegistry fields in API comments
- Breaking changes require migration guide
- Update docs/ for user-facing changes
- Keep managed and unmanaged component behavior explicit in the PR description

## Code Structure

- `apis/quay/v1/` - CRD types
- `controllers/quay/` - reconciliation logic
- `pkg/` - component management (postgres, redis, clair, etc.)
- `kustomize/` - deployment manifests
- `test/chainsaw/` - cluster behavior tests
