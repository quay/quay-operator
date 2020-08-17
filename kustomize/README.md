## Goal

As someone deploying Quay to Kubernetes, I want to provide a config bundle directory with the services I want and receive k8s manifests which are `kubectl create` ready in order to declaratively manage my Quay deployment.

## Anti-Goals

- Use any form of templating (writing k8s YAML is enough)
- Application lifecycle management (beyond what native k8s controllers provide)

## Usage

This repository is intended as a _template_ for your unique Quay installation. 
Create a copy, add your secrets, values, and certs, choose which features you want enabled, and deploy!

Be sure to `git commit` your changes to adhere to configuration-as-code! 
You can use `git-crypt` to protect your secrets, like database credentials and access keys.

### Prerequisites 

- `go` v1.14+
- `kubectl` and a Kubernetes cluster

### Deploying Quay

1. Use the `app/` directory to "kustomize" your Quay deployment.
  - Uncomment items under `resources` to include them in your deployment
  - Add other Quay config fields to `bundle/config.yaml`
  - Add custom SSL cert files to `bundle/ssl.key` and `bundle/ssl.cert` and uncomment them under `secretGenerator` in `kustomization.yaml`
  - Add extra SSL cert files needed by Quay to talk to external services to `bundle` directory and add them to `secretGenerator` in `kustomization.yaml`

2. When you are ready to generate the final deployment files, run the following:
```sh
$ mkdir ./output
$ go run main.go
```

This is a small Go program which internally uses `kustomize` as a library, then properly formats the `quay-config-secret`.

3. Now you can simply use `kubectl` or any other Kubernetes client to deploy Quay:
```sh
$ kubectl create -n quay-enterprise -f ./output
```

### Modifying Configuration Values

Say you want to make changes to your Quay's configuration, like adding a new superuser. 

1. Add/modify the desired field in `app/bundle/config.yaml`
2. Commit your changes to source control to maintain a history and be able to revert if needed:
```sh
$ git add . && git commit -m "adding new superuser"
```

3. Generate new Kubernetes manifests by running:
```sh
$ rm -rf output/* && go run main.go
```

Note that if you look in the `output/` directory, you can see an new Quay `Secret` has been created with a _different_ suffix hash in `metadata.name`.
The Quay `Deployment` has been updated to reference this new `Secret`, which is what will trigger a rolling deploy. 

4. Apply the new resources to your cluster:
```sh
$ kubectl apply -n quay-enterprise -f ./output
```

Note that this will trigger a rolling deploy. Because the old config `Secret` is still present on the cluster (because we created a new one rather than updating it),
we can easily roll back the deploy to point at the previous secret in case something goes wrong.

### Adding/Removing a Managed Service

Say you deployed a basic Quay initially, and you've heard how awesome container security scanning is with [Clair](https://github.com/quay/clair)! Let's add it to your deployment.

1. Update `app/kustomization.yaml` to include the `clair` component:
```yaml
components:
  - ...
  - ../components/clair
```

2. Commit your changes to source control to maintain a history and be able to revert if needed:
```sh
$ git add . && git commit -m "adding clair security scanner"
```

3. Generate new Kubernetes manifests by running:
```sh
$ rm -rf output/* && go run main.go
```

Note that if you look in the `output/` directory, you can see new manifests for Clair have been created, as well as an updated Quay `Secret` with the Clair-specific fields (`FEATURE_SECURITY_SCANNER`).

4. Apply the new resources to your cluster:
```sh
$ kubectl apply -n quay-enterprise -f ./output
```

Note that this will trigger a rolling deploy. Because the old config `Secret` is still present on the cluster (because we created a new one rather than updating it),
we can easily roll back the deploy to point at the previous secret in case something goes wrong.

#### Replacing a Managed Service

Now, say that your team is cutting-edge and has a fork of Clair that adds new features and want to point your Quay to use it. The purpose of this project is to provide an opinionated install of Quay, so there is no supported way to configure managed services like Clair. However, you are welcome to bring your own service.
Say you have your custom Clair service deployed separately at `http://custom-clair`, you can point Quay to it.

1. Update `app/kustomization.yaml` to remove the `clair` component
2. Update `app/bundle/config.yaml` to include Clair-specific fields to point to your service:
```yaml
FEATURE_SECURITY_SCANNER: true
SECURITY_SCANNER_V4_ENDPOINT: http://custom-clair
SECURITY_SCANNER_V4_NAMESPACE_WHITELIST: [admin]
```

3. Generate new Kubernetes manifests by running:
```sh
$ rm -rf output/* && go run main.go
```

Note that if you look in the `output/` directory, you can see the Clair manifests have been removed.

4. Apply the new resources to your cluster:
```sh
$ kubectl apply -n quay-enterprise -f ./output
```

Note that this will trigger a rolling deploy. Because the old config `Secret` is still present on the cluster (because we created a new one rather than updating it),
we can easily roll back the deploy to point at the previous secret in case something goes wrong.

Your Quay instance is now pointing to your custom Clair service, which must be managed by you.

### Teardown

Teardown is as simple as running:
```sh
$ kubectl delete -n quay-enterprise -f ./output
```

### Known Limitations

- This tool does not provide validation of the resulting `quay-config-secret`
- If you choose to use an unmanaged external service (database/storage/Redis/Clair), you must add the appropriate `<CONFIG_FIELD>: <value>` entries to `app/bundle/config.yaml`
- On OCP, you need to run `oc adm policy add-scc-to-user anyuid system:serviceaccount:quay-enterprise:default` before deploying
- Need to manually point DNS to the created Quay `Service` with `type: LoadBalancer` (ensure it matches `SERVER_HOSTNAME` in `config.yaml`)
- Cannot modify configuration of managed services (like Clair) because this is an opinionated deployment

### Future Work

- [x] Use `kustomize` as a library instead of CLI
- [x] Use [Kustomize `components`](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/components.md) instead of `variants` for more DRY code
- [ ] Zero downtime upgrades with database migrations
- [ ] Use other Operators to provide external services (like CrunchyDB Postgres Operator, Redis Operator, etc...)
- [ ] Refactor into Go module which can be imported by other tools
- [ ] Quay Operator which provides application lifecycle management using this tool
- [ ] Add `ownerReferences` to all created resources for easy tracking and cleanup
