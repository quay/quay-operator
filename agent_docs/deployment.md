# Deployment

## Local Development

```bash
# Install CRDs into cluster
make install

# Run controller locally
make run

# Run with namespace restriction
go run main.go --namespace <your-namespace>

# Run without resource requests (useful for resource-constrained clusters)
SKIP_RESOURCE_REQUESTS=true make run
```

## Quick Deployment on OpenShift

```bash
# 1. Install NooBaa object storage via ODF operator
./hack/storage.sh

# 2. Deploy the operator from published catalog
./hack/deploy.sh

# 3. Create a QuayRegistry
oc create -n <your-namespace> -f ./config/samples/managed.quayregistry.yaml
```

### Environment Variables for hack/deploy.sh

- `TAG` - Catalog image tag (default: `3.6-unstable`)
- `CATALOG_IMAGE` - Full catalog image reference
- `OPERATOR_PKG_NAME` - Operator package name (default: `quay-operator-test`)

## CRD Management

```bash
# Install CRDs
make install

# Uninstall CRDs
make uninstall

# Apply CRDs manually
kubectl apply -f ./bundle/upstream/manifests/*.crd.yaml

# Generate CRDs from Go types
make manifests
```

## Building Container Images

```bash
# Build operator image
make docker-build IMG=<registry>/<namespace>/quay-operator:dev

# Push operator image
make docker-push IMG=<registry>/<namespace>/quay-operator:dev
```

### Building Custom Catalog

1. Build and push operator image
2. Update image in `bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml`
3. Build operator bundle:
   ```bash
   docker build -t <registry>/quay-operator-bundle:dev -f ./bundle/Dockerfile ./bundle
   docker push <registry>/quay-operator-bundle:dev
   ```
4. Build operator index:
   ```bash
   cd bundle/upstream
   opm index add --bundles <registry>/quay-operator-bundle:dev --tag <registry>/quay-operator-index:dev
   docker push <registry>/quay-operator-index:dev
   ```
5. Update `bundle/quay-operator.catalogsource.yaml` with index image
6. Apply CatalogSource:
   ```bash
   kubectl create -n openshift-marketplace -f ./bundle/quay-operator.catalogsource.yaml
   ```

## Sample QuayRegistry Resources

Located in `config/samples/`:

- `managed.quayregistry.yaml` - Fully managed deployment
- Additional samples for various configurations

## Component Image Overrides

Override component images via environment variables:

```bash
export RELATED_IMAGE_COMPONENT_QUAY=quay.io/projectquay/quay:v3.10.0
export RELATED_IMAGE_COMPONENT_CLAIR=quay.io/projectquay/clair:v4.7.0
export RELATED_IMAGE_COMPONENT_POSTGRES=quay.io/sclorg/postgresql-13-c9s:latest
export RELATED_IMAGE_COMPONENT_REDIS=docker.io/library/redis:7
make run
```
