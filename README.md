# Quay Operator

![CI](https://github.com/quay/quay-operator/workflows/CI/badge.svg?branch=master)

Opinionated deployment of [Quay container registry](https://github.com/quay/quay) on Kubernetes.

## Welcome

The original version of the quay-operator is available on the _v1_ branch. The next generation operator, known as TNG or v2, is developed on _master_ branch.

## Principles

- Kubernetes is a powerful platform with an abundance of existing and future features. Always prefer to leverage something Kubernetes does better than try to implement it again.
- Favor declarative application management to improve everyone's sanity and understanding of the state.
- Make things simple by default, but always allow diving deeper to discover the details of what is going on.

## Getting Started

This Operator can be installed on any Kubernetes cluster running the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager). Simply create the provided `CatalogSource` to make the package available on the cluster, then create the `Subscription` to install it.

**Create the `CatalogSource`**:
```sh
$ kubectl create -n openshift-marketplace -f ./deploy/quay-operator.catalogsource.yaml
```

**Wait a few seconds for the package to become available**:
```sh
$ kubectl get packagemanifest --all-namespaces | grep quay
```

**Create the `OperatorGroup`**:

NOTE: By default, the `targetNamespaces` field is specified to target the _quay-enterprise_ namespace. Update this value if the namespace the operator is deployed within differs. 

```sh
$ kubectl create -n <your-namespace> -f ./deploy/quay-operator.operatorgroup.yaml
```

**Create the `Subscription` to install the Operator**:
```sh
$ kubectl create -n <your-namespace> -f ./deploy/quay-operator.subscription.yaml
```

### Using the Operator

#### Batteries-included, zero-config

**Install RHOCS Operator using OperatorHub:**

**Create `NooBaa` object in `openshift-storage` namespace:**
```sh
$ kubectl create -n openshift-storage -f ./kustomize/components/objectstorage/quay-datastore.noobaa.yaml
```

**Wait a few minutes for Noobaa to be `phase: Ready`:**
```sh
$ kubectl get -n openshift-storage noobaas noobaa -w
NAME     MGMT-ENDPOINTS              S3-ENDPOINTS                IMAGE                                                                                                            PHASE   AGE
noobaa   [https://10.0.32.3:30318]   [https://10.0.32.3:31958]   registry.redhat.io/ocs4/mcg-core-rhel8@sha256:56624aa7dd4ca178c1887343c7445a9425a841600b1309f6deace37ce6b8678d   Ready   3d18h
```

**Create `QuayRegistry` instance:**
```sh
$ kubectl create -n <your-namespace> -f ./config/samples/managed.quayregistry.yaml
```

## Community

- Mailing list: [quay-sig@googlegroups.com](https://groups.google.com/forum/#!forum/quay-sig)
- IRC: #quay on [freenode.net](https://webchat.freenode.net/)
- Bug tracking: https://issues.redhat.com/projects/PROJQUAY/summary
- Security issues: [security@redhat.com](security@redhat.com)

## Contributing

Pull requests and bug reports are always welcome!

### Local Development

#### Prerequisites

- `KUBECONFIG` environment variable set in shell to valid k8s cluster
- `go`
- `kubectl`
- `kubebuilder`
- `docker`

**Create the `QuayRegistry` CRD**:
```sh
$ kubectl create -f ./config/crd/bases/
```

**Run the controller**:
```sh
$ go run main.go
```

**Tests**:
```sh
$ go test -v ./...
```

**Building custom `CatalogSource`**:

1. Build and push the Quay Operator container:

```sh
$ docker build -t quay.io/<namespace>/quay-operator:dev .
$ docker push quay.io/<namespace>/quay-operator:dev
```

2. Replace the `image` field in `deploy/manifests/quay-operator/0.0.1/quay-operator.clusterserviceversion.yaml` with the image above.

3. Build and push the Quay Operator `CatalogSource` container:

```sh
$ cd deploy
$ docker build -t quay.io/<namespace>/quay-operator-catalog:dev .
$ docker push quay.io/<namespace>/quay-operator-catalog:dev
```

4. Replace the `spec.image` field in `deploy/quay-operator.catalogsource.yaml` with the image above.

5. Create the custom `CatalogSource`:

```sh
$ kubectl create -n openshift-marketplace -f ./deploy/quay-operator.catalogsource.yaml
```
