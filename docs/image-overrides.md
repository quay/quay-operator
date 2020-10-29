# Image Overrides

The container images used by the managed components deployed by the Quay Operator can be overridden. 

**NOTE**: This should only be done for development, testing, and debugging as it not guaranteed that all components will be compatible when overriding the defaults included in the Operator.

### Environment Variables

The following environment variables are used in the Operator to override component images:

| Environment Variable | Component |
|---|---|
| `RELATED_IMAGE_COMPONENT_QUAY` | `base` |
| `RELATED_IMAGE_COMPONENT_CLAIR` | `clair` |
| `RELATED_IMAGE_COMPONENT_POSTGRES` | `postgres` + `clair` (database) |
| `RELATED_IMAGE_COMPONENT_REDIS` | `redis` |

**NOTE:** Override images **must** be referenced by _manifest_ (`@sha256:`), not by _tag_ (`:latest`).

### Applying Overrides to a Running Operator

When the Quay Operator is installed in a cluster via the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager) (OLM), the managed component container images can be easily overridden by modifying the `ClusterServiceVersion` object, which is OLM's representation of a running Operator in the cluster. Find the Quay Operator's `ClusterServiceVersion` either by using a Kubernetes UI or `kubectl`:

```sh
$ kubectl get clusterserviceversions -n <your-namespace>
```

Using the UI, `kubectl edit`, or any other method, modify the Quay `ClusterServiceVersion` to include the environment variables outlined above to point to the override images:

**JSONPath**: `spec.install.spec.deployments[0].spec.template.spec.containers[0].env`
```yaml
- name: RELATED_IMAGE_COMPONENT_QUAY
  value: quay.io/projectquay/quay@sha256:c35f5af964431673f4ff5c9e90bdf45f19e38b8742b5903d41c10cc7f6339a6d
- name: RELATED_IMAGE_COMPONENT_CLAIR
  value: quay.io/projectquay/clair@sha256:70c99feceb4c0973540d22e740659cd8d616775d3ad1c1698ddf71d0221f3ce6
- name: RELATED_IMAGE_COMPONENT_POSTGRES
  value: centos/postgresql-10-centos7@sha256:de1560cb35e5ec643e7b3a772ebaac8e3a7a2a8e8271d9e91ff023539b4dfb33
- name: RELATED_IMAGE_COMPONENT_REDIS
  value: centos/redis-32-centos7@sha256:06dbb609484330ec6be6090109f1fa16e936afcf975d1cbc5fff3e6c7cae7542
```
