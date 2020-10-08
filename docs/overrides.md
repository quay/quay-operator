# Overrides

By leveraging a [feature of Kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/image.md), we can override the container images that are deployed by the Quay Operator as part of managed components for a `QuayRegistry`.

**NOTE**: This is not supported and should only be used for development/testing as it will likely break between updates.

### Images

The following images can be overwritten:

- `quay.io/projquay/quay`
- `quay.io/projquay/clair`
- `postgres`
- `redis`

### Create Override ConfigMap

Create `ConfigMap` called `quay-dev-kustomize` in the same namespace that the Operator is installed in with the following contents in the `kustomization.yaml` key:

```yaml
# Overlay variant for "dev".
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonAnnotations:
  quay-version: dev
bases:
  - ../../../tmp
images:
  # Replace `newName` with your custom image or leave it the same.
  - name: quay.io/projectquay/quay
    newName: quay.io/alecmerdler/quay
    newTag: dev
  - name: quay.io/projectquay/clair
    newName: quay.io/alecmerdler/clair
    newTag: dev
  - name: postgres
    newName: postgres
  - name: redis
    newName: redis
```

Either create this before installing the Operator, or simply restart the Operator `Pod` so that it mounts in the `ConfigMap` as a volume.

### Desired Version

Now when you create a `QuayRegistry` with the `spec.desiredVersion` set to `dev`, it will "inflate" to use these custom images:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: override
spec:
  desiredVersion: dev
```

**NOTE**: This is done at the Operator level, so every `QuayRegistry` will be deployed using these overrides.