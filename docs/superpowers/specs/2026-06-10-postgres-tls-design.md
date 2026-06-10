# PostgreSQL TLS Support — Design Spec

**Date:** 2026-06-10
**Status:** Approved
**JIRA:** PROJQUAY-11215
**Goal:** Encrypt PostgreSQL connections for managed postgres and clairpostgres components.

## Context

Today, both Quay→postgres and Clair→clairpostgres connections are unencrypted:
- Quay's `DB_URI` is generated as `postgresql://user:pass@host:5432/db` with no `sslmode` parameter (defaults to `prefer` in libpq, but no server cert is available so it falls back to unencrypted)
- Clair's connection string is hardcoded with `sslmode=disable` in `pkg/kustomize/secrets.go`
- The PostgreSQL deployment has no TLS cert volumes or `ssl` directives in `postgresql.conf`

## Scope

- **In scope:** Managed `postgres` and `clairpostgres` components only
- **Out of scope:** Unmanaged PostgreSQL (user handles their own TLS)

## Design Decisions

### Cert Provisioning — Two Paths

**Path 1 — Service CA (OpenShift, preferred):**
- Annotate the PG Service with `service.beta.openshift.io/serving-cert-secret-name: <name>-<component>-pg-tls`
- The OpenShift service CA operator auto-generates a `kubernetes.io/tls` Secret signed by the cluster's service CA
- Clients (Quay app, Clair) trust the server cert via the `service-ca.crt` bundle from the `<name>-cluster-service-ca` ConfigMap (which the operator already manages)

**Path 2 — User-provided Secret (fallback / vanilla K8s):**
- User provides a `kubernetes.io/tls` Secret containing the PG server cert/key
- User provides the corresponding CA cert so clients can verify the server
- The operator mounts the server cert into the PG pod and the CA into client pods

### SSL Mode

- Clients connect with `sslmode=verify-ca` when PG TLS is enabled
- This verifies the server cert is signed by a trusted CA but does not check hostname (appropriate for synthetic cluster DNS names)
- The CA root cert path is passed via `sslrootcert` in the connection string (Quay `DB_URI`) or connection parameters (Clair `connstring`)

### Opt-in Mechanism

- PG TLS is **opt-in** via a new spec field on the postgres/clairpostgres component overrides
- PG TLS stays off unless the user explicitly enables it — no surprise behavior change on operator upgrade
- When enabled on OpenShift, the operator uses service CA by default; if a user-provided PG TLS secret is also specified, the user-provided cert takes precedence

### Graceful Fallback

- On KinD/vanilla K8s without service CA and without a user-provided cert: if PG TLS is requested but no cert source is available, the operator sets `RolloutBlocked` with a clear error message
- The postgres deployment does not start with TLS enabled unless a valid cert is mounted

## Behavioral Rules (for spec files)

### PostgreSQL Server-Side TLS

1. When PG TLS is enabled for a managed postgres component, the operator configures the PostgreSQL server to accept TLS connections by adding `ssl = on`, `ssl_cert_file`, and `ssl_key_file` directives to `postgresql.conf.sample`.
2. The TLS cert/key are mounted as a volume in the postgres Deployment from either the service CA-generated Secret or a user-provided Secret.
3. The PostgreSQL server cert Secret name follows the pattern `<registry-name>-<component>-pg-tls` (e.g., `example-quay-database-pg-tls`, `example-clair-postgres-pg-tls`).

### Client-Side Connection Changes

4. When PG TLS is enabled for managed postgres, the operator generates `DB_URI` with `?sslmode=verify-ca&sslrootcert=<ca-path>` appended.
5. When PG TLS is enabled for managed clairpostgres, the Clair connection string changes from `sslmode=disable` to `sslmode=verify-ca sslrootcert=<ca-path>`.
6. The CA root cert is mounted into Quay app and Clair pods from the `cluster-service-ca` ConfigMap (service CA path) or a user-provided CA Secret.

### Cert Source Priority

7. If the user provides a PG TLS secret reference, that cert takes precedence over service CA — even on OpenShift.
8. If PG TLS is enabled and the service CA is available (OpenShift), the operator annotates the PG Service to trigger cert generation automatically.
9. If PG TLS is enabled but no cert source is available (no service CA, no user-provided cert), the operator blocks reconciliation with `RolloutBlocked` condition and reason `ConfigInvalid`.

### Independence of PG TLS and Quay Endpoint TLS

10. PG TLS (database connection encryption) is independent of the `tls` component (Quay HTTPS endpoint). Enabling one does not require or imply the other.
11. The CA trust chain for PG TLS may differ from the Quay endpoint TLS CA. Service CA-signed PG certs are trusted via `service-ca.crt`, while the Quay endpoint may use the cluster wildcard cert or a user-provided cert with a different CA.

## Spec Files to Update

| File | Changes |
|------|---------|
| `what/component-management.md` | New "Database TLS" section after "External TLS Secret" with rules for the opt-in field, cert provisioning, and validation |
| `what/config-bundle.md` | Update DB_URI generation rules (rule 4) to document sslmode parameter injection when PG TLS is enabled |
| `what/tls.md` | New "Database TLS" section documenting PG TLS as distinct from endpoint TLS, cert sources, CA trust |
| `how/reconciliation.md` | Update context gathering step 4 to include PG TLS cert detection |
| `glossary.md` | Add "Database TLS" and "Service CA" terms |

## Backward Compatibility

| Scenario | Before | After | Breaking? |
|----------|--------|-------|-----------|
| PG TLS not set (default) | Unencrypted | Unencrypted | No |
| PG TLS enabled, service CA available | N/A | Encrypted with service CA cert | No (new feature) |
| PG TLS enabled, user-provided cert | N/A | Encrypted with user cert | No (new feature) |
| PG TLS enabled, no cert available | N/A | RolloutBlocked error | No (explicit opt-in) |
| Unmanaged postgres | Unencrypted (user's choice) | Unchanged | No |
