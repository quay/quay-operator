# Config Bundle

The config bundle secret contains Quay's `config.yaml` and optional certificates. The operator merges user-provided configuration with operator-generated values to produce a rendered config secret that is mounted into Quay pods.

## Behavioral Rules

### Config Bundle Creation

1. If `spec.configBundleSecret` is empty, the operator creates a new Secret with `GenerateName: "<name>-config-bundle-"` containing a base `config.yaml` with Quay defaults.
2. The operator updates `spec.configBundleSecret` to point to the newly created secret and requeues.

### Merge Semantics

3. The operator generates a rendered config secret (name pattern: `<name>-quay-config-secret-<hash>`) by merging the user's config bundle with operator-generated values.
4. For managed components, the operator overwrites the corresponding field group in the rendered config. The field group mapping is:
   - `clair` → `SecurityScanner`
   - `postgres` → `Database`
   - `redis` → `Redis`
   - `objectstorage` → `DistributedStorage`
   - `route` → `HostSettings`
   - `mirror` → `RepoMirror`
   - `clairpostgres`, `horizontalpodautoscaler`, `monitoring`, `tls`, `quay` → no field group (no config override)
5. For unmanaged components, the user-provided config for that field group is preserved as-is in the rendered config.
6. Components that support user config even when managed (`route`, `mirror`, `redis`) allow the user to set config values that coexist with operator-generated values.

### Config Validation

7. The middleware (`pkg/middleware`) validates the rendered config against Quay's JSON schema and ensures required fields are present for managed components.
8. If validation fails, the `RolloutBlocked` condition is set with reason `ConfigInvalid`.

### Secret Rotation

9. Each rendered config secret has a unique name containing a content hash. When config changes, a new secret is created with a different hash.
10. Old rendered config secrets (matching prefix `<name>-quay-config-secret-*`) are NOT deleted until the `quay-app` Deployment has fully rolled out — all desired replicas must be both updated and available. This prevents running pods from losing their mounted config volume (PROJQUAY-9157).
11. The rollout check compares `Deployment.Status.ObservedGeneration` against `Deployment.Generation` to avoid reading stale counters from a previous revision.
12. If the `quay-app` Deployment does not yet exist (first reconcile), rollout is considered complete and orphaned secrets are cleaned up immediately.

### Managed Keys Secret

13. Generated secrets (DATABASE_SECRET_KEY, SECRET_KEY, DB_URI, DB_ROOT_PW, SECURITY_SCANNER_V4_PSK, CLAIR_DB_USER, CLAIR_DB_PASSWORD, CLAIR_DB_ROOT_PW, CLAIR_DB_NAME) are persisted in a separate `<name>-quay-registry-managed-secret-keys` Secret.
14. The managed keys secret survives config bundle changes. It is read at the start of each reconcile to restore generated values.
15. A legacy managed keys format (pre-3.7.0) is supported via label-based lookup as a fallback when the named secret is not found.

### Deterministic Secret Generation

16. Config secret content must be deterministic given the same inputs. Non-deterministic output (e.g., random ordering of map keys, timestamps in generated content) causes unnecessary pod rollouts because the content hash changes.

### Config Secret Stripping

17. The middleware strips the following keys from the rendered config secret before it is mounted to pods:
    - `ssl.cert`, `ssl.key` — prevents user-provided TLS certs from affecting Quay's generated NGINX config
    - `clair-ssl.key`, `clair-ssl.crt` — same for Clair TLS
    - `extra_ca_cert_*` — extra CA certs are mounted separately via the extra-ca-certs volume

## Constraints

- Config secrets use `GenerateName`, so they cannot be predicted or referenced before creation.
- The merge is one-directional: operator-generated values overwrite user values for managed field groups. There is no deep merge.
