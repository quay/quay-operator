# Components

## QuayRegistry CRD

The `QuayRegistry` custom resource defines a Quay deployment:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: example
spec:
  configBundleSecret: quay-config-bundle  # Optional: custom config
  components:
    - kind: postgres
      managed: true
      overrides:
        volumeSize: 100Gi
```

## Managed Components

Components can be `managed: true` (operator handles lifecycle) or `managed: false` (user provides external service).

| Component | Description | Required | Default |
|-----------|-------------|----------|---------|
| `quay` | Quay application | Yes (always managed) | managed |
| `postgres` | Quay database | Yes | managed |
| `redis` | Build logs, locking | Yes | managed |
| `objectstorage` | Image blob storage | Yes | managed (if ObjectBucketClaim API available) |
| `route` | External access | Yes (OpenShift) | managed (if Route API available) |
| `tls` | TLS certificates | Yes | managed (if no custom certs provided) |
| `clair` | Vulnerability scanner | No | managed |
| `clairpostgres` | Clair database | No | managed |
| `horizontalpodautoscaler` | HPA for Quay/Clair/Mirror | No | managed |
| `mirror` | Repository mirroring | No | managed |
| `monitoring` | Prometheus metrics | No | managed (if Prometheus API available) |

## Component Overrides

Overrides customize managed component resources.

### Supported Overrides by Component

| Override | quay | clair | mirror | postgres | clairpostgres | redis |
|----------|------|-------|--------|----------|---------------|-------|
| `volumeSize` | - | Yes | - | Yes | Yes | - |
| `storageClassName` | - | - | - | Yes | Yes | - |
| `env` | Yes | Yes | Yes | Yes | Yes | Yes |
| `replicas` | Yes | Yes | Yes | - | - | - |
| `affinity` | Yes | Yes | Yes | - | - | - |
| `resources` | Yes | Yes | Yes | Yes | Yes | - |
| `labels` | Yes | Yes | Yes | Yes | Yes | Yes |
| `annotations` | Yes | Yes | Yes | Yes | Yes | Yes |

### Override Examples

```yaml
spec:
  components:
    # PostgreSQL with custom storage
    - kind: postgres
      managed: true
      overrides:
        volumeSize: 100Gi
        storageClassName: fast-storage
        env:
          - name: POSTGRESQL_MAX_CONNECTIONS
            value: "500"
        resources:
          requests:
            memory: 4Gi
            cpu: "2"

    # Quay with custom replicas and affinity
    - kind: quay
      managed: true
      overrides:
        replicas: 3
        affinity:
          podAntiAffinity:
            preferredDuringSchedulingIgnoredDuringExecution:
              - weight: 100
                podAffinityTerm:
                  labelSelector:
                    matchLabels:
                      quay-component: quay-app
                  topologyKey: kubernetes.io/hostname
```

## Override Validation

Defined in `apis/quay/v1/quayregistry_types.go`:

- Cannot set overrides on unmanaged components
- Cannot override replicas when HPA is managed (except to 0 for scale-down)
- Volume/storage overrides only allowed on components with persistent storage

## Adding a New Component

1. Add `ComponentKind` constant in `apis/quay/v1/quayregistry_types.go`
2. Add to `AllComponents` slice
3. Add Kustomize manifests in `kustomize/components/<name>/`
4. Create status checker in `pkg/cmpstatus/<name>.go`
5. Update `pkg/kustomize/kustomize.go` to handle the component
6. Add e2e tests in `e2e/`

## Unmanaged Component Configuration

When a component is unmanaged, provide configuration in the config bundle secret:

```yaml
# For unmanaged postgres
DATABASE_SECRET_KEY: <base64-encoded>
DB_URI: postgresql://user:pass@host:5432/quay

# For unmanaged redis
BUILDLOGS_REDIS:
  host: redis.example.com
  port: 6379

# For unmanaged objectstorage
DISTRIBUTED_STORAGE_CONFIG:
  default:
    - S3Storage
    - host: s3.amazonaws.com
      # ... S3 config
```
