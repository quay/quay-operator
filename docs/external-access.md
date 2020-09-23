# External Access

A container registry isn't very useful if you can't use it! The Quay Operator supports external access to the Quay registry using common Kubernetes APIs.

## LoadBalancer Service

When running on Kubernetes, external access is provided by creating a `Service` of `type: LoadBalancer`. This should work for most clusters deployed on cloud providers. After creating the `QuayRegistry`, find the Quay `Service`.

```sh
$ kubectl get services -n <namespace>
NAME                    TYPE        CLUSTER-IP       EXTERNAL-IP          PORT(S)             AGE
some-quay               ClusterIP   172.30.143.199   34.123.133.39        443/TCP,9091/TCP    23h
```

You can then configure your DNS provider to point the `SERVER_HOSTNAME` to that IP address.

## OpenShift Routes

When running on OpenShift, the `Routes` API is available and will automatically be used as a managed component.  After creating the `QuayRegistry`, the external access point can be found in the `status` block of the `QuayRegistry`:

```yaml
status:
  registryEndpoint: some-quay.my-namespace.apps.mycluster.com
```

### Default Hostname and TLS

By default, a `Route` will be created with the default generated hostname and TLS edge termination using the cluster TLS.

### Custom Hostname and TLS

If you want to access Quay using a custom hostname and bring your own TLS certificate/key pair, first create a `Secret` which contains the following:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-config-bundle
data:
  config.yaml: <must include SERVER_HOSTNAME field with your custom hostname>
  ssl.cert: <your TLS certificate>
  ssl.key: <your TLS key>
```

Then, create a `QuayRegistry` which references the created `Secret`:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: some-quay
spec:
  configBundleSecret: my-config-bundle
```

Make sure your DNS provider creates a CNAME record for `SERVER_HOSTNAME` to the OpenShift canonical router.

### Disabling Route Component

To prevent the Operator from creating a `Route`, mark the component as unmanaged in the `QuayRegistry`:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: some-quay
spec:
  components:
    - kind: route
      managed: false
```

Note that you are now responsible for creating a `Route`, `Service`, or `Ingress` in order to access the Quay instance and that whatever DNS you use must match the `SERVER_HOSTNAME` in the Quay config.
