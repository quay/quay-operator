# Development 

The local development experience of the Quay Operator should be easy and straightforward. Please file an issue if you disagree!

### Controller

1. Ensure that the `KUBECONFIG` environment variable is set for your Kubernetes cluster (same file that `kubectl` uses).

2. Install the CRDs for the operator

```sh
$ kubectl apply -f ./bundle/upstream/manifests/*.crd.yaml
```

2. Run the controller:
```sh
$ go run main.go --namespace <your-namespace>
```

### Tests

Prerequsites: Install `kubebuilder`

```sh
$ go test -v ./...
```
### Config Editor

The Quay Operator deploys a "config-editor" server which provides a rich UI experience for modifying Quay's `config.yaml` bundle. The "config-editor" server then sends a payload to an endpoint exposed by the Operator pod itself, which triggers a re-deploy. Obviously, this won't work during local development when the controller is running on your own machine but deploying to a remote Kubernetes cluster. To solve this, you can use a tool like `ngrok` to expose a local server to the internet.

1. Start forwarding using `ngrok` and copy the public forwarding URL (looks like `http://988e36df98ca.ngrok.io`):
```sh
$ ngrok http 7071
```

2. Run the Operator controller locally and set the environment variable using the `ngrok` URL:
```sh
$ DEV_OPERATOR_ENDPOINT=http://988e36df98ca.ngrok.io go run main.go --namespace <your-namespace>
```

3. Access the config editor locally by port-forwarding the service endpoint

```shell script
$ kubectl port-forward -n <quay-namespace> svc/<name>-quay-config-editor 8080
```

Point the browser to localhost:8080 to access the config UI

The credentials for the Config editor can be found in the secret `<prefix>-config-secret-<random-suffix>`
eg: `test-quay-config-secret-tk88ffkdmt`

### Quay.io Branch 

For running the `quayio` legacy branch of Quay, there are some extra steps to get everything working. 
(NOTE: there are better ways to do this, but this is the fastest/easiest).

1. Modify the `users` field of the `SecurityContextContstraints` for `anyuid` to include the Quay `ServiceAccount`:

```yaml
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: anyuid
users:
  - 'system:serviceaccount:<namespace>:<quayregistry-name>-quay-app'
```

2. Create a `RoleBinding` to allow the Quay `ServiceAccount` to view `Secrets` in its namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: quay-view
  namespace: <namespace>
subjects:
  - kind: ServiceAccount
    name: <quayregistry-name>-quay-app
    namespace: <namespace>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  # NOTE: The `view` role does not include `Secrets`...
  name: edit
```

3. The `config.yaml` entry supplied in `spec.configBundleSecret` must include the following field (from legacy behavior requiring a user to exist in the database before startup):

```yaml
SERVICE_LOG_ACCOUNT_ID: 12345
```

4. With managed `objectstorage` using RHOCS, the cluster service CA certificate needs to be included manually in `spec.configBundleSecret` (due to legacy behavior of Quay) under the key `extra_ca_certs_cluster-service-ca.crt`.
