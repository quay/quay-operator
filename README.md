# Quay Operator

![CI](https://github.com/quay/quay-operator/workflows/CI/badge.svg?branch=master)

Opinionated deployment of [Quay container registry](https://github.com/quay/quay) on Kubernetes.

## Principles

- Kubernetes is a powerful platform with an abundance of existing and future features. Always prefer to leverage something Kubernetes does better than try to implement it again.
- Favor declarative application management to improve everyone's sanity and understanding of the state.
- Make things simple by default, but always allow diving deeper to discover the details of what is going on.

## Getting Started 

This Operator can be installed on any Kubernetes cluster running the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager). Simply create the provided `CatalogSource` to make the package available on the cluster, then create the `Subscription` to install it.

**If running on OpenShift**:
```sh
$ oc adm policy add-scc-to-user anyuid system:serviceaccount:<your-namespace>:default
```

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
