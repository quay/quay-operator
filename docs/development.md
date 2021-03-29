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
