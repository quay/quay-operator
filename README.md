# quay-operator

[![Docker Repository on Quay](https://quay.io/repository/redhat-cop/quay-operator/status "Docker Repository on Quay")](https://quay.io/repository/redhat-cop/quay-operator)

Operator to manage the lifecycle of [Quay](https://www.openshift.com/products/quay).

## Overview

This repository contains the functionality to provision a Quay ecosystem. Quay is supported by a number of other components, and thus a CustomResourceDefinition called `QuayEcosystem` is used to define the entire architecture. 

The following components are supported to be maintained by the Operator:

* Quay
* Redis

## Provisioning a Quay Ecosystem

### Deploy the Operator

Quay requires that it be deployed in a namespace called `quay-enterprise`.

```
$ oc new-project quay-enterprise
```

Deploy the cluster resources. Given that a number of elevated permissions are required to resources at a cluster scope the account you are currently logged in must have elevated rights

```
$ oc create -f deploy/crds/cop_v1alpha1_quayecosystem_crd.yaml
$ oc create -f deploy/service_account.yaml
$ oc create -f deploy/cluster_role.yaml
$ oc create -f deploy/cluster_role_binding.yaml
$ oc create -f deploy/role.yaml
$ oc create -f deploy/role_binding.yaml
$ oc create -f deploy/operator.yaml
```


### Deploy a Quay Ecosystem

Create a pull secret to retrieve Quay images from quay.io

```
$ oc create secret generic coreos-pull-secret --from-file=".dockerconfigjson=<location of docker.json file>" --type='kubernetes.io/dockerconfigjson'
```

Create a custom resource to deploy the Quay ecosystem. The following is an example of a `QuayEcosystem` custom resource to support a deployment of the Quay ecosystem

```
apiVersion: cop.redhat.com/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  imagePullSecretName: coreos-pull-secret
```

You can also run the following command to create the `QuayEnterprise` custom resource

```
$ oc create -f deploy/crds/cop_v1alpha1_quayecosystem_cr.yaml
```

#### Persistence Support

MySQL or PostgreSQL can be deployed to provide persistence for quay.

The following QuayEcosystem custom resource can be used to provision Quay along with a backing MySQL database:

```
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  imagePullSecretName: coreos-pull-secret
  quay:
    database:
      type: mysql
```

## Local Development

Execute the following steps to develop the functionality locally. It is recommended that development be done using a cluster with `cluster-admin` permissions. 

Clone the repository, then resolve all depdendencies using `dep`

```
$ dep ensure
```

Using the [operator-sdk](https://github.com/operator-framework/operator-sdk), run the operator locally

```
$ operator-sdk up local --namespace=quay-enterprise
```