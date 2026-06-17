# TLS Security Profile Inheritance (PROJQUAY-10705)

## Overview

The Quay Operator automatically inherits the cluster-wide TLS security profile from the OpenShift `APIServer` resource and applies it to Quay's nginx configuration. This ensures Quay respects the same TLS policy as the rest of the cluster without requiring manual configuration.

Mirror workers inherit the same settings automatically via the shared `config.yaml`.

### Behavior Summary

| Scenario | Result |
|----------|--------|
| OpenShift cluster with no TLS profile set (nil) | Defaults to **Intermediate** (TLSv1.2 + TLSv1.3) |
| OpenShift cluster with explicit profile (Old/Intermediate/Modern/Custom) | Translates to matching `SSL_PROTOCOLS` and `SSL_CIPHERS` |
| User sets `SSL_PROTOCOLS` or `SSL_CIPHERS` in `configBundleSecret` | **User values take precedence** — cluster profile is not applied |
| Vanilla Kubernetes (no `config.openshift.io` API) | Gracefully skipped — no TLS settings injected |

### Profile Mapping

| OpenShift Profile | SSL_PROTOCOLS | Ciphers |
|-------------------|---------------|---------|
| Old | TLSv1, TLSv1.1, TLSv1.2, TLSv1.3 | Full legacy set |
| Intermediate (default) | TLSv1.2, TLSv1.3 | Modern + legacy ECDHE/DHE ciphers |
| Modern | TLSv1.3 | TLS 1.3 ciphers only |
| Custom | Based on `minTLSVersion` | User-specified cipher list |

## Architecture

```
                    ┌─────────────────────────┐
                    │  APIServer "cluster"     │
                    │  .spec.tlsSecurityProfile│
                    └───────────┬──────────────┘
                                │ GET (read-only)
                    ┌───────────▼──────────────┐
                    │  checkTLSSecurityProfile()│ controllers/quay/tls.go
                    │  translateTLSProfile()    │
                    └───────────┬──────────────┘
                                │ populates
                    ┌───────────▼──────────────┐
                    │  QuayRegistryContext      │ pkg/context/context.go
                    │  .SSLProtocols            │
                    │  .SSLCiphers              │
                    └───────────┬──────────────┘
                                │ consumed by
                    ┌───────────▼──────────────┐
                    │  Inflate()                │ pkg/kustomize/kustomize.go
                    │  → config.yaml            │
                    │    SSL_PROTOCOLS: [...]    │
                    │    SSL_CIPHERS: [...]      │
                    └───────────────────────────┘
```

### Files Changed

| File | Purpose |
|------|---------|
| `controllers/quay/tls.go` | TLS profile detection and translation logic |
| `controllers/quay/tls_test.go` | Unit + integration tests |
| `controllers/quay/quayregistry_controller.go` | Calls `checkTLSSecurityProfile()` during reconcile |
| `pkg/context/context.go` | `SSLProtocols` and `SSLCiphers` fields on context |
| `pkg/kustomize/kustomize.go` | Injects TLS settings into generated `config.yaml` |
| `main.go` | Registers `configv1` scheme |
| `config/rbac/role.yaml` | RBAC: `get`/`watch` on `config.openshift.io/apiservers` |

### Error Handling

If the operator cannot read the `APIServer` resource for a reason other than "not found" or "API not registered", reconciliation sets a `RolloutBlocked` condition with reason `ConfigInvalid`. This prevents deploying Quay with an unknown TLS posture.

## Testing Guide

### Unit Tests

Run the TLS-specific unit tests:

```bash
go test ./controllers/quay/ -run 'TestTranslateTLS|TestCheckTLS|TestTls' -v
```

This covers:
- `TestTranslateTLSProfile` — All profile types (nil, Old, Intermediate, Modern, Custom, Custom-with-nil-spec)
- `TestTlsVersionToProtocols` — MinTLSVersion → nginx protocol string mapping
- `TestTlsCiphersToString` — Cipher list → OpenSSL colon-separated format
- `TestCheckTLSSecurityProfile_UserOverride` — User-set SSL_PROTOCOLS/SSL_CIPHERS blocks inheritance
- `TestCheckTLSSecurityProfile_VanillaK8s` — Graceful no-op when configv1 API is absent
- `TestCheckTLSSecurityProfile_WithAPIServer` — Modern profile populates context correctly
- `TestCheckTLSSecurityProfile_NilProfile` — Nil profile defaults to Intermediate

### E2E Testing on OpenShift (CRC or cluster)

#### Prerequisites

- An OpenShift cluster (CRC works) with `cluster-admin` access
- The Quay Operator CRD installed (`oc get crd quayregistries.quay.redhat.com`)
- The operator binary built (`make manager`)

#### Test 1: Nil Profile → Intermediate Default

This validates the default behavior when no cluster TLS profile is configured.

```bash
# 1. Confirm no TLS profile is set
oc get apiserver cluster -o jsonpath='{.spec.tlsSecurityProfile}'; echo
# Expected: empty output

# 2. Create namespace and config bundle
oc create namespace quay-e2e-tls
cat <<'EOF' | oc apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: quay-config-bundle
  namespace: quay-e2e-tls
type: Opaque
stringData:
  config.yaml: |
    FEATURE_USER_INITIALIZE: true
    SUPER_USERS:
      - quayadmin
    DISTRIBUTED_STORAGE_CONFIG:
      default:
        - LocalStorage
        - storage_path: /datastorage/registry
    DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
      - default
    DISTRIBUTED_STORAGE_PREFERENCE:
      - default
EOF

# 3. Create QuayRegistry
cat <<'EOF' | oc apply -f -
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: tls-test
  namespace: quay-e2e-tls
spec:
  configBundleSecret: quay-config-bundle
  components:
    - kind: clair
      managed: false
    - kind: postgres
      managed: true
    - kind: redis
      managed: true
    - kind: objectstorage
      managed: false
    - kind: mirror
      managed: false
    - kind: monitoring
      managed: false
    - kind: horizontalpodautoscaler
      managed: false
    - kind: quay
      managed: true
    - kind: tls
      managed: true
    - kind: route
      managed: true
    - kind: clairpostgres
      managed: false
EOF

# 4. Wait for config secret to be generated
sleep 15
CONFIG_SECRET=$(oc get secrets -n quay-e2e-tls -o name | grep quay-config-secret)

# 5. Verify TLS settings
oc get $CONFIG_SECRET -n quay-e2e-tls \
  -o jsonpath='{.data.config\.yaml}' | base64 -d | grep -A5 SSL_

# Expected:
#   SSL_PROTOCOLS: [TLSv1.2, TLSv1.3]
#   SSL_CIPHERS: [TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384, ...]  (11 ciphers)
```

#### Test 2: Modern Profile Propagation

This validates that changing the cluster TLS profile is picked up by the operator.

```bash
# 1. Set cluster TLS profile to Modern
oc patch apiserver cluster --type=merge \
  -p '{"spec":{"tlsSecurityProfile":{"type":"Modern","modern":{}}}}'

# NOTE: This will restart the API server. Wait for it to stabilize.
# On CRC this typically takes 10-30 seconds.

# 2. Trigger a reconcile (annotation change)
oc annotate quayregistry tls-test -n quay-e2e-tls \
  tls-test-trigger="$(date +%s)" --overwrite

# 3. Wait and check the new config secret
sleep 15
CONFIG_SECRET=$(oc get secrets -n quay-e2e-tls -o name | grep quay-config-secret)
oc get $CONFIG_SECRET -n quay-e2e-tls \
  -o jsonpath='{.data.config\.yaml}' | base64 -d | grep -A5 SSL_

# Expected:
#   SSL_PROTOCOLS: [TLSv1.3]
#   SSL_CIPHERS: [TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384, TLS_CHACHA20_POLY1305_SHA256]
```

#### Test 3: User Override Takes Precedence

This validates that user-provided TLS settings in `configBundleSecret` are not overwritten.

```bash
# 1. Update config bundle with explicit SSL settings
cat <<'EOF' | oc apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: quay-config-bundle
  namespace: quay-e2e-tls
type: Opaque
stringData:
  config.yaml: |
    FEATURE_USER_INITIALIZE: true
    SUPER_USERS:
      - quayadmin
    DISTRIBUTED_STORAGE_CONFIG:
      default:
        - LocalStorage
        - storage_path: /datastorage/registry
    DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
      - default
    DISTRIBUTED_STORAGE_PREFERENCE:
      - default
    SSL_PROTOCOLS:
      - TLSv1.2
    SSL_CIPHERS:
      - ECDHE-RSA-AES128-GCM-SHA256
      - ECDHE-RSA-AES256-GCM-SHA384
EOF

# 2. Trigger reconcile
oc annotate quayregistry tls-test -n quay-e2e-tls \
  tls-test-trigger="$(date +%s)" --overwrite

# 3. Verify user values preserved (cluster is still Modern = TLSv1.3)
sleep 15
CONFIG_SECRET=$(oc get secrets -n quay-e2e-tls -o name | grep quay-config-secret)
oc get $CONFIG_SECRET -n quay-e2e-tls \
  -o jsonpath='{.data.config\.yaml}' | base64 -d | grep -A5 SSL_

# Expected (user values, NOT cluster Modern values):
#   SSL_PROTOCOLS: [TLSv1.2]
#   SSL_CIPHERS: [ECDHE-RSA-AES128-GCM-SHA256, ECDHE-RSA-AES256-GCM-SHA384]
```

#### Cleanup

```bash
# Restore cluster TLS profile
oc patch apiserver cluster --type=json \
  -p='[{"op":"remove","path":"/spec/tlsSecurityProfile"}]'

# Delete test resources
oc delete quayregistry tls-test -n quay-e2e-tls
oc delete namespace quay-e2e-tls
```

### Running the Operator Locally for Testing

If the operator is not installed via OLM, run it locally:

```bash
make manager
SKIP_RESOURCE_REQUESTS=true bin/manager --metrics-addr=:9090
```

Use `--metrics-addr=:9090` if port 8080 is occupied (e.g., by CRC's gvproxy).

## Validated Results

The following results were obtained on a CRC v4.21.0 cluster on 2026-03-24:

| Test | Cluster Profile | User Override | Generated SSL_PROTOCOLS | Generated SSL_CIPHERS | Result |
|------|----------------|---------------|------------------------|-----------------------|--------|
| Nil → Intermediate | nil | none | `[TLSv1.2, TLSv1.3]` | 11 Intermediate ciphers | PASS |
| Modern propagation | Modern | none | `[TLSv1.3]` | 3 TLS 1.3 ciphers | PASS |
| User override | Modern | `[TLSv1.2]` + 2 ciphers | `[TLSv1.2]` | User's 2 ciphers | PASS |

## Known Limitations

1. **No reactive watch on APIServer**: The operator reads the TLS profile during each reconcile loop but does not watch the `APIServer` resource for changes. A cluster TLS profile change will only take effect on the next reconcile triggered by another event (e.g., QuayRegistry spec change, owned resource update, or the periodic requeue).

2. **Data format round-trip**: TLS protocols and ciphers are stored as joined strings in `QuayRegistryContext` (`"TLSv1.2 TLSv1.3"` / `"cipher1:cipher2"`) and split back into arrays when injected into `config.yaml`. This works correctly but could be simplified by storing `[]string` directly.

3. **Duplicate scheme registration**: `configv1.Install(scheme)` is called in both `main.go` `init()` and `SetupWithManager()`. Both calls are idempotent so this is harmless, but could be consolidated.
