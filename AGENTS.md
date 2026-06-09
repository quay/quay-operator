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

## Local Development (KinD)

For local development without an existing cluster:

```bash
# Create a local KinD cluster with all dependencies
make local-dev-up

# Start the operator (in a separate terminal)
SKIP_RESOURCE_REQUESTS=true make run

# Check environment status
make local-dev-status

# Tear down when done
make local-dev-down
```

Prerequisites: `go`, `podman` (or `docker`), `kind`, `openssl`.
The setup creates a 3-node KinD cluster with Garage S3 for object
storage, self-signed TLS, and a ready-to-use QuayRegistry CR.
After running the operator, Quay is accessible at `https://127.0.0.1:30443`.

Optional auth providers can be added with `LOCAL_DEV_OPTS`:

```bash
# With LDAP (389 Directory Server) and Keycloak (OIDC)
LOCAL_DEV_OPTS="--ldap --keycloak" make local-dev-up
```

**LDAP users** (password: `password`): admin, user1, quayadmin, readonly, testuser, admin\_ldap, testuser\_ldap, readonly\_ldap.
**Keycloak OIDC users** (password: `password`): admin\_oidc, testuser\_oidc, readonly\_oidc.
Keycloak admin console: `http://127.0.0.1:30080` (admin/admin).

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
