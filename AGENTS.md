# Quay Operator - AI Agent Guide

Kubernetes operator for deploying [Quay container registry](https://github.com/quay/quay) on OpenShift/Kubernetes.

## Tech Stack

- **Language**: Go 1.23+
- **Framework**: controller-runtime (Kubebuilder)
- **CRD**: `QuayRegistry` in `quay.redhat.com/v1`
- **Dependencies**: PostgreSQL, Redis, Clair, object storage (NooBaa/S3)

## Development Workflow

```bash
# Build
make manager

# Run locally against cluster (requires KUBECONFIG)
make run

# Run without resource requests (dev mode)
SKIP_RESOURCE_REQUESTS=true make run

# Test
make test

# Format and vet
make fmt && make vet

# Generate CRDs and code
make generate && make manifests
```

## Project Structure

```
apis/quay/v1/          # CRD type definitions (QuayRegistry)
controllers/quay/      # Reconciliation logic
pkg/kustomize/         # Manifest generation from Kustomize
pkg/cmpstatus/         # Component health evaluation
pkg/context/           # Runtime state (cluster capabilities, TLS, storage)
kustomize/             # Base Kustomize manifests by component
e2e/                   # E2E tests using kuttl
hack/                  # Deployment and utility scripts
```

## Documentation by Topic

Consult these files when working on specific areas:

| Topic | File | When to Read |
|-------|------|--------------|
| Architecture | `agent_docs/architecture.md` | Understanding reconciliation flow, controllers, status evaluation |
| Testing | `agent_docs/testing.md` | Running tests, writing e2e tests, kuttl patterns |
| Deployment | `agent_docs/deployment.md` | CRD management, OpenShift deployment, image building |
| Components | `agent_docs/components.md` | Managed components, overrides, adding new components |

## Contributing

All changes require a referenced [PROJQUAY Jira](https://issues.redhat.com/projects/PROJQUAY/issues).

Commit message format:
```
<subsystem>: <what changed> (PROJQUAY-####)

<why this change was made>
```

## Key Conventions

- Maintain existing code style
- Run `make fmt` before committing
- Keep CRD backward compatible
- Test component changes with e2e tests in `e2e/`
