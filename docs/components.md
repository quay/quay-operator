# Components

Quay is a powerful container registry platform and as a result, requires a decent number of dependencies. These include a database, object storage, Redis, and others. The Quay Operator manages an opinionated deployment of Quay and its dependencies on Kubernetes. These dependencies are treated as _components_ and are configured through the `QuayRegistry` API.

### List of Components

This is the full list of components ([code](https://github.com/quay/quay-operator/tree/master/kustomize/components)):
- `postgres`
- `clair`
- `redis`
- `objectstorage`
- `horizontalpodautoscaler`
- `mirror`
- `route`
- `monitoring`
- `cache`

### API

The `spec.components` field of the `QuayRegistry` object configures components. Each component contains two fields: `kind` - the name of the component, and `managed` - boolean whether the component lifecycle is handled by the Operator. By default (omitting this field), all components are _managed_ and will be autofilled upon reconciliation for visibility:

```yaml
spec:
  components:
    - kind: postgres
      managed: true
    ...
```

### Component Config

Configuring Quay application containers to use components happens through the `config.yaml` file which is mounted into the container. The Quay Operator will automatically populate the necessary `config.yaml` values for any components marked as `managed: true`, which have been codified using ["field groups"](https://github.com/quay/config-tool/tree/master/pkg/lib/fieldgroups).

If a user wants to use an unanaged component (like an external database), they would need to provide those database-related fields to Quay using a `config.yaml`. The Operator receives this in the form of a Kubernetes `Secret` using the `spec.configBundleSecret` field on the `QuayRegistry` API.

### Example

1. Create a `Secret` with the necessary database fields in a `config.yaml`:

`config.yaml`:
```yaml
DB_URI: postgresql://test-quay-database:postgres@test-quay-database:5432/test-quay-database 
```

```sh
$ kubectl create secret generic --from-file config.yaml=./config.yaml test-config-bundle
```

2. Create a `QuayRegistry` which marks `postgres` component as unmanaged and references the created `Secret`:

`test.quayregistry.yaml`
```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: test
spec:
  configBundleSecret: test-config-bundle
  components:
    - kind: postgres
      managed: false
```

The deployed Quay application will now use the external database.

### Note on the cache component
If 

```yaml
spec:
  components:
  - kind: redis
    managed: false
  - kind: cache
    managed: true
```

is set in the `QuayRegistry` custom resource (`cache` managed, `redis` unmanaged), then the operator will not reconcile properly as `cache` component is only meant to be managed by the operator if internal Redis instance is used. When external Redis instance is in place (`redis` component is unmanaged), then `cache` must be set to unmanaged for reconciliation to continue. Caching can then be configured by adding the configuration to the init config bundle's `config.yaml` file. An example for external Redis:

```yaml
DATA_MODEL_CACHE_CONFIG:
  engine: redis
  redis_config:
    primary:
      host: redis-hostname
      port: 6379
      password: redis-password
      ssl: true
  repository_blob_cache_ttl: 120s
  catalog_page_cache_ttl: 120s
  active_repo_tags_cache_ttl: 120s
  value_size_limit: 2MiB
```
