# Test Migration Guide: quay-tests → quay-operator (Chainsaw)

## Context

The QE team maintains operator tests in `quay-tests/new-quay-operator-tests/` using Ginkgo v2. These tests cover 35+ scenarios for the Quay Operator but live in a separate repo, making them harder to maintain alongside operator code changes. We're porting **net-new coverage only** into the quay-operator's existing Chainsaw e2e framework.

**Goal**: No coverage loss. Every quay-tests scenario is mapped below to one of: COVERED (already in Chainsaw), PORT (will be ported), SKIP (justified exclusion). OLM install/upgrade testing (#29, #35, #37) is handled by Prow CI steps, not Chainsaw.

## Source Files (quay-tests)

- `new-quay-operator-tests/test/extended/oquay/quayregistry.go` — 27 test cases (single-ns + all-ns modes)
- `new-quay-operator-tests/test/extended/oquay/upgrade.go` — 5 upgrade test cases
- `new-quay-operator-tests/test/extended/oquay/util.go` — shared utilities
- `new-quay-operator-tests/test/extended/oquay/utils_*.go` — OLM helpers

## Target (quay-operator Chainsaw)

- `test/chainsaw/reconcile/` — core reconcile lifecycle
- `test/chainsaw/hpa/` — HPA management
- `test/chainsaw/ca_rotation/` — CA cert rotation (destructive)
- `test/chainsaw/unmanaged_postgres/` — external database

---

## Complete Test Migration Map

### Part 1: Single NS Mode (quayregistry.go lines 21-1931)

| # | QE Test ID | Description | Chainsaw Status | Action |
|---|-----------|-------------|-----------------|--------|
| 1 | OCP-21167 | Unmanaged AWS S3 storage | **COVERED** — `reconcile` uses Garage S3 via config bundle; same operator code path (unmanaged objectstorage) | None |
| 2 | OCP-42375 | Managed route | **COVERED** — `reconcile` creates managed route, verifies annotation/label overrides and TLS termination | None |
| 3 | OCP-42377 | Managed clair | **COVERED** — `reconcile` creates managed clair, verifies clair pods + updater health | None |
| 4 | OCP-42385 | Managed redis | **COVERED** — `reconcile` creates managed redis as part of all-managed registry | None |
| 5 | OCP-42391 | Managed route + TLS | **COVERED** — `reconcile` creates managed route+TLS on OpenShift | None |
| 6 | OCP-42404 | Managed mirror | **COVERED** — `reconcile` has full mirror lifecycle (manage/unmanage/scale/remanage) | None |
| 7 | OCP-42399 | Managed HPA | **COVERED** — `hpa` test creates managed HPA then exercises lifecycle | None |
| 8 | OCP-32404 | Managed objectstorage (NooBaa) | **COVERED** — `reconcile` on OpenShift sets `objectstorage: managed=true` via `values-openshift.yaml`; KinD uses unmanaged (Garage S3) | None |
| 9 | OCP-32391 | Managed postgres | **COVERED** — `reconcile` creates managed postgres, verifies migration job timing | None |
| 10 | OCP-42376 | **Unmanaged Redis** | **NOT COVERED** | **PORT** |
| 11 | OCP-42285 | Unmanaged PostgreSQL | **COVERED** — `unmanaged_postgres` test | None |
| 12 | OCP-42403 | Unmanaged HPA + mirror | **COVERED** — `hpa` covers unmanaged HPA; `reconcile` covers unmanaged mirror | None |
| 13 | OCP-42378 | **Unmanaged Clair** | **NOT COVERED** | **PORT** |
| 14 | OCP-42396 | **Unmanaged route + unmanaged TLS** | **NOT COVERED** | **PORT** (OCP only) |
| 15 | OCP-42393 | **Managed route + unmanaged TLS (with certs)** | **NOT COVERED** | **PORT** (OCP only) |
| 16 | OCP-42387 | User-provided TLS cert/key | **COVERED BY #15** — identical flow to OCP-42393 | Merge into #15 |
| 17 | OCP-42374 | **Override quay hostname (unmanaged TLS)** | **NOT COVERED** | **PORT** (OCP only, merge into route/tls test) |
| 18 | OCP-42395 | **Managed route + unmanaged TLS without certs (NEGATIVE)** | **NOT COVERED** | **PORT** (OCP only) |
| 19 | OCP-71993 | **Resource requests/limits overrides** | **NOT COVERED** | **PORT** |
| 20 | OCP-72156 | **Remove resource limitation** | **NOT COVERED** | **PORT** (step in resource_overrides test) |
| 21 | OCP-69866 | HPA managed->unmanaged + minReplicas change | **PARTIALLY** — `hpa` test covers unmanage/bump/remanage but not PROJQUAY-6474 pod stability | **ENHANCE** hpa test |
| 22 | OCP-73445 | Unmanaged HPA + custom user HPA | **PARTIALLY** — `hpa` covers unmanage but not user-created HPA resource | **ENHANCE** hpa test |
| 23 | OCP-46883 | **Override DB volumes (PVC size 70Gi)** | **NOT COVERED** | **PORT** |
| 24 | OCP-49387 | **Managed clair + unmanaged clairpostgres** | **NOT COVERED** | **PORT** (combine with unmanaged_clair) |
| 25 | OCP-53302 | Anti-affinity override (requiredDuring) | **BUG** — operator middleware annotation-to-ComponentKind mapping broken (`quayapp` != `quay`), affinity overrides silently dropped. `preferred` in reconcile comes from kustomize base, not override. | Skip (operator bug, needs PROJQUAY fix) |
| 26 | OCP-85810 | **Custom StorageClass (valid, 3.16+)** | **NOT COVERED** | **PORT** |
| 27 | OCP-85811 | **Invalid StorageClass (negative, 3.16+)** | **NOT COVERED** | **PORT** |

### Part 2: All NS Mode (quayregistry.go lines 1934-2319)

| # | QE Test ID | Description | Chainsaw Status | Action |
|---|-----------|-------------|-----------------|--------|
| 28 | OCP-40694 | All managed incl monitoring | **MOSTLY COVERED** — `reconcile` tests all managed except monitoring | **PORT** monitoring assertion (OCP only) |
| 29 | OCP-42752 | Verify operator images from registry.redhat.io | **SKIP** — covered by Prow CI `quay-install-operator-bundle` step | None |
| 30 | OCP-42479 | Delete quay deployment -> status unhealthy | **PARTIALLY** — `reconcile` deletes quay-app but doesn't assert status error message | **ENHANCE** existing reconcile step |
| 31 | OCP-42838 | **Delete clair -> status unhealthy** | **COVERED** — `reconcile/delete-clair-app` step | None |
| 32 | OCP-42841 | **Delete mirror -> status unhealthy** | **COVERED** — `reconcile/delete-mirror` step | None |
| 33 | OCP-42845 | **Delete redis -> status unhealthy** | **COVERED** — `reconcile/delete-redis` step | None |
| 34 | OCP-42844 | **Delete postgres -> status unhealthy** | **COVERED** — `reconcile/delete-postgres` step | None |

### Part 3: Upgrade Tests (upgrade.go)

| # | QE Test ID | Description | Chainsaw Status | Action |
|---|-----------|-------------|-----------------|--------|
| 35 | OCP-20934 | Quay operator upgrade via OLM (NooBaa) | **SKIP** — covered by Prow CI `e2e-upgrade` job | None |
| 36 | OCP-42610 | Stub for 20934 | N/A | **SKIP** (stub only) |
| 37 | OCP-74056 | Upgrade with parameter overrides | **SKIP** — covered by Prow CI `e2e-upgrade` job | None |
| 38 | OCP-26302 | CSO operator upgrade | N/A | **SKIP** (separate operator) |
| 39 | OCP-42453 | QBO operator upgrade | N/A | **SKIP** (separate operator) |

### Migration Summary

- **COVERED (no action)**: 16 scenarios (#1-9, #11-12, #16, #31-34)
- **PORT (net-new)**: 13 scenarios (#10, #13-15, #17-20, #23-24, #26-28)
- **ENHANCE (augment existing)**: 3 scenarios (#21-22 into hpa, #30 into reconcile)
- **SKIP (justified)**: 7 scenarios (#25 operator bug, #29/#35/#37 Prow CI, #36 stub, #38-39 separate operators)
- **Total**: 39 scenarios mapped, 0 unaccounted

---

## New Chainsaw Test Directories

### 1. `test/chainsaw/unmanaged_clair/` — Covers #13, #24

**What it tests**: QuayRegistry with `clair: managed=false` and `clairpostgres: managed=false`

**Steps**:
1. Create config bundle secret (platform-aware, same pattern as `unmanaged_postgres`)
2. Apply QuayRegistry with clair+clairpostgres unmanaged
3. Assert all 14 status conditions True
4. Script: verify NO `clair-app` deployment, NO `clair-postgres` deployment
5. Script: verify `quay-app` pods are running

**Pattern**: Follow `unmanaged_postgres/` exactly. No external service needed (unlike unmanaged_postgres which deploys an external DB) — we simply don't manage clair.

**Files**:
- `chainsaw-test.yaml`
- `00-create-quay-registry.yaml`
- `00-assert-status.yaml`

### 2. `test/chainsaw/unmanaged_redis/` — Covers #10

**What it tests**: QuayRegistry with `redis: managed=false` and external Redis provided via config bundle

**Steps**:
1. Deploy external Redis (Deployment + Service) — minimal redis container
2. Assert Redis pod ready
3. Create config bundle with `BUILDLOGS_REDIS` and `USER_EVENTS_REDIS` pointing to external service
4. Apply QuayRegistry with `redis: managed=false`
5. Assert all 14 status conditions True
6. Script: verify NO `quay-redis` deployment exists
7. Script: verify `quay-app` pods are running

**Pattern**: Follow `unmanaged_postgres/` — deploy external service first, then registry.

**Files**:
- `chainsaw-test.yaml`
- `00-deploy-redis.yaml`, `00-assert-redis.yaml`
- `01-create-quay-registry.yaml`, `01-assert-status.yaml`

### 3. `test/chainsaw/unmanaged_route_tls/` — Covers #14, #15, #17, #18 (OpenShift only)

**What it tests**: Route and TLS managed/unmanaged combinations

**Steps**:
1. **Guard**: Skip on KinD (`kubectl api-resources | grep route.openshift.io`)
2. **Step A (OCP-42393)**: Managed route + unmanaged TLS with user certs
   - Script: get cluster base domain, generate self-signed cert matching route hostname
   - Create config bundle with `SERVER_HOSTNAME`, `ssl.cert`, `ssl.key`
   - Apply QuayRegistry: `route: managed=true, tls: managed=false`
   - Assert status Available
   - Script: verify route exists with `tls.termination: passthrough`
3. **Step B (OCP-42396 + OCP-42374)**: Unmanaged route + unmanaged TLS
   - Delete step A registry
   - Create new config bundle with custom `SERVER_HOSTNAME` (hostname override)
   - Apply QuayRegistry: `route: managed=false, tls: managed=false`
   - Assert status Available
   - Script: verify NO route created by operator
4. **Step C (OCP-42395, NEGATIVE)**: Managed route + unmanaged TLS **without** certs
   - Create config bundle with `SERVER_HOSTNAME` but NO `ssl.cert`/`ssl.key`
   - Apply QuayRegistry: `route: managed=true, tls: managed=false`
   - Script: wait 60s, verify quay-app pods do NOT start

**Files**:
- `chainsaw-test.yaml`
- `00-create-managed-route-unmanaged-tls.yaml`, `00-assert-status.yaml`
- `01-create-unmanaged-route-tls.yaml`, `01-assert-status.yaml`
- `02-create-negative-no-certs.yaml`

### 4. `test/chainsaw/resource_overrides/` — Covers #19, #20, #23 (#25 skipped due to operator bug)

**What it tests**: Resource requests/limits, PVC volume size overrides

**Steps**:
1. **Step A (OCP-71993)**: Resource requests/limits
   - Apply QuayRegistry with explicit `resources.limits` and `resources.requests` on quay, clair, mirror, postgres, clairpostgres
   - Assert each Deployment's container resources match spec
   - Script: validate CPU/memory values via jsonpath
2. **Step B (OCP-72156)**: Remove resource limitation
   - Patch QuayRegistry to remove resource limits (empty overrides)
   - Assert deployments revert to defaults
3. **Step C (OCP-46883)**: PVC volume override (70Gi)
   - Volume overrides are set inline with resource overrides (`volumeSize: 70Gi` on postgres and clairpostgres)
   - Assert PVC capacity matches 70Gi

**Note**: OCP-53302 (anti-affinity `requiredDuringScheduling` overrides) is skipped. The operator middleware annotation-to-ComponentKind mapping is broken (`quayapp` != `quay`), so affinity overrides are silently dropped. See `resource_overrides/chainsaw-test.yaml` for details.

**Files**:
- `chainsaw-test.yaml`
- `00-create-quay-registry.yaml`, `00-assert-status.yaml`

### 5. `test/chainsaw/custom_storageclass/` — Covers #26, #27 (3.16+ only)

**What it tests**: Custom StorageClass for PVCs (valid + invalid negative test)

**Steps**:
1. Script: create a StorageClass (platform-aware provisioner: `ebs.csi.aws.com` on OCP, `rancher.io/local-path` on KinD)
2. **Step A (OCP-85810, positive)**: Apply QuayRegistry with `storageClassName` on postgres and clairpostgres
   - Assert status Available
   - Script: verify PVCs use the custom StorageClass
3. **Step B (OCP-85811, negative)**: Apply QuayRegistry with `storageClassName: invalid-does-not-exist`
   - Script: wait 3m, verify PVCs are Pending
   - Script: verify postgres pods are Pending

**Files**:
- `chainsaw-test.yaml` (includes inline StorageClass creation)
- `00-create-valid-sc-registry.yaml`, `00-assert-status.yaml`
- `01-create-invalid-sc-registry.yaml`

### Component Health (#31-34)

Scenarios #31-34 (delete component, verify status reports unavailable) are covered by the `reconcile/` test's `delete-quay-app`, `delete-clair-app`, `delete-mirror`, `delete-redis`, and `delete-postgres` steps. No separate `component_health/` directory needed.

---

## Enhancements to Existing Tests

### Enhance `reconcile/` step "delete-quay-app" (#30)

Add assertion after the quay-app deletion step that verifies the QuayRegistry status contains the expected error message before the operator recreates the deployment.

```bash
# After deletion, verify status reports the issue
for i in $(seq 1 30); do
  MSG=$(kubectl get quayregistry reconcile -n $NAMESPACE \
    -o jsonpath='{.status.conditions[?(@.type=="Available")].message}' 2>/dev/null || true)
  if echo "${MSG}" | grep -qi "awaiting\|zero replicas"; then
    echo "PASS: Status reports component unavailable: ${MSG}"
    break
  fi
  sleep 2
done
```

### Enhance `reconcile/` for monitoring (#28)

Add OpenShift-only script step to verify monitoring component status is true when `managedMonitoring` is true. No-op on KinD.

### Enhance `hpa/` for managed->unmanaged pod stability (#21, #22)

Add steps after the existing unmanage-hpa step:
- Verify existing quay-app pods remain stable (not terminating) after HPA is unmanaged
- Create a user-defined HPA with `minReplicas: 3`, verify 3 pods come up without churn

---

## Makefile Changes

**File**: `test/chainsaw/Makefile`

The `test-e2e-kind` target excludes OpenShift-only tests:
```makefile
test-e2e-kind: chainsaw
	$(CHAINSAW) test ... --exclude-test-regex "/ca.rotation|/hpa|/unmanaged.route" --assert-timeout 20m
```

The `values-openshift.yaml` and `values-kind.yaml` files include an `openshift` flag for platform-specific step gating.

---

## Implementation Order

| Phase | Test Directory | Scenarios | Complexity | Depends On |
|-------|---------------|-----------|------------|------------|
| 1 | `unmanaged_clair/` | #13, #24 | Low | None -- follows unmanaged_postgres pattern |
| 2 | `unmanaged_redis/` | #10 | Low-Med | None -- deploys external Redis |
| 3 | `reconcile/` (delete steps) | #31-34 | Done | Folded into reconcile test |
| 4 | `resource_overrides/` | #19, #20, #23, #25 | Medium | None |
| 5 | `custom_storageclass/` | #26, #27 | Medium | None |
| 6 | Enhance `reconcile/` + `hpa/` | #21, #22, #28, #30 | Low | Existing tests |
| 7 | `unmanaged_route_tls/` | #14, #15, #17, #18 | Med-High | OpenShift cluster |

---

## Verification Plan

After each phase, verify:

1. **KinD tests pass**: `hack/setup-kind-e2e.sh && make test-e2e-kind` -- new tests that work on KinD should be included
2. **Exclusion patterns work**: New OCP-only tests are properly excluded from `test-e2e-kind`
3. **OpenShift tests pass** (if cluster available): `make test-e2e` with operator running locally
4. **No regressions**: Existing `reconcile`, `hpa`, `ca_rotation`, `unmanaged_postgres` tests still pass
5. **CI workflow**: `.github/workflows/e2e-kind.yaml` continues to pass (excludes OCP-only tests)

---

## Patterns to Reuse

- **Config bundle creation**: Copy script pattern from `unmanaged_postgres/chainsaw-test.yaml` lines 40-101
- **Status assertion**: Copy `00-assert-status.yaml` from `reconcile/` (all 14 conditions)
- **OpenShift detection**: `kubectl api-resources | grep route.openshift.io` pattern from `reconcile/`
- **External service deployment**: Copy Deployment+Service pattern from `unmanaged_postgres/00-deploy-postgres.yaml`
