# quay-operator

[![Build Status](https://travis-ci.org/redhat-cop/quay-operator.svg?branch=master)](https://travis-ci.org/redhat-cop/quay-operator) [![Docker Repository on Quay](https://quay.io/repository/redhat-cop/quay-operator/status "Docker Repository on Quay")](https://quay.io/repository/redhat-cop/quay-operator)

Operator to manage the lifecycle of [Quay](https://www.openshift.com/products/quay).

## Overview

This repository contains the functionality to provision a Quay ecosystem. Quay is supported by a number of other components, and thus a CustomResourceDefinition called `QuayEcosystem` is used to define the entire architecture. 

The following components are supported to be maintained by the Operator:

* Quay Enterprise
* Redis
* PostgreSQL

## Provisioning a Quay Ecosystem

### Deploy the Operator

Quay requires that it be deployed in a namespace called `quay-enterprise`.

```
$ oc new-project quay-enterprise
```

Deploy the cluster resources. Given that a number of elevated permissions are required to resources at a cluster scope the account you are currently logged in must have elevated rights

```
$ oc create -f deploy/crds/redhatcop_v1alpha1_quayecosystem_crd.yaml
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
$ oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=<location of docker.json file>" --type='kubernetes.io/dockerconfigjson'
```

Create a custom resource to deploy the Quay ecosystem. The following is an example of a `QuayEcosystem` custom resource to support a deployment of the Quay ecosystem

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
  redis:
    imagePullSecretName: redhat-pull-secret
```

You can also run the following command to create the `QuayEnterprise` custom resource

```
$ oc create -f deploy/crds/redhatcop_v1alpha1_quayecosystem_cr.yaml
```

This will automatically create and configure Quay Enterprise, PostgreSQL and Redis. The credentials that can be used to access Quay Enterprise is described in the following section.

### Credentials

There are several locations within the Quay ecosystem of tools where sensitive information is stored/used. The Operator supports credentials either provided by the user as _Secrets_, or using defaults.

#### Quay Superuser

During the installation process, a superuser is created in Quay Enterprise with administrative rights. The credentials for this user can be specified or leverage the following default values:

Username: `quay` 
Password: `password`
Email: `quay@redhat.com`

To specify an alternate value, create a secret as shown below:

```
oc create secret generic <secret_name> --from-literal=superuser-username=<username> --from-literal=superuser-password=<password> --from-literal=superuser-email=<email>
```

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  imagePullSecretName: redhat-pull-secret
  quay:
    superuserCredentialsName: <secret_name>
    imagePullSecretName: redhat-pull-secret
  redis:
    imagePullSecretName: redhat-pull-secret
```

#### Quay Configuration

A dedicated deployment of Quay Enterprise is used to manage the configuration. By default, the following values are used:

Username: `quayconfig`
Password: `quay`

While the username cannot be changed, the password can be defined within a secret. To define a password for the Quay Configuration, execute the following command to create a secret:

```
oc create secret generic <secret_name> --from-literal=config-app-password=<password>
```

Reference the name of the secret in the _QuayEcosystem_ custom resource as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    configSecretName: <secret_name>
    imagePullSecretName: redhat-pull-secret
  redis:
    imagePullSecretName: redhat-pull-secret
```

### Persistent Storage using PostgreSQL

The PostgreSQL relational database is used as the persistent store for Quay Enterprise. PostgreSQL can either be deployed by the operator within the namespace or leverage an existing instance. The determination of whether to provision an instance or not within the current namespace depends on whether the `server` property within the `QuayEcosystem` is defined. 

The following options are a portion of the available options to configure the PostgreSQL database:

| Property | Description | 
| --------- | ---------- |
| `image` | Location of the database image |
| `volumeSize` | Size of the volume in Kubernetes capacity units |
| `memory` | Amount of memory in Kubernetes resource units |
| `cpu` | Amount of cpu in Kubernetes resource units |

Define the values as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    database:
      volumeSize: 10Gi
  redis:
    imagePullSecretName: redhat-pull-secret
```

#### Specifying Credentials

The credentials for accessing the server can be specified through a _Secret_ or when being provisioned by the operator, leverage the following default values:

```
Username: `quay`
Password: `quay`
Root Password: `quayAdmin`
Database Name: `quay`
```

To define alternate values, create a secret as shown below:

```
oc create secret generic <secret_name> --from-literal=database-username=<username> --from-literal=database-password=<password> --from-literal=database-root-password=<root-password> --from-literal=database-name=<database-name>
```

Reference the name of the secret in the _QuayEcosystem_ custom resource as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    database:
      credentialsSecretName: <secret_name>
  redis:
    imagePullSecretName: redhat-pull-secret
```

### Registry Storage

Quay supports multiple storage backends. The quay operator supports aiding in the facilitation of certain storage backends. The following backends are currently supported:

* Local

#### Configuring Local Storage

Local storage references a local directory within the Quay pod for which image metadata is stored. The configuration is specified by using the `registryStorage` parameters underneath the `quay` property. By default, a _PersistentVolumeClaim_ is created to support the registry (no configuration necessary). However, you may specify any of the following parameters as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryStorage:
      local:
        persistentVolumeAccessMode:
          - ReadWriteOnce
        persistentVolumeSize: 10Gi
  redis:
    imagePullSecretName: redhat-pull-secret
```

To disable the creation of a _PersistentVolumeClaim_ and instead use an _EmptyDir_ volume, specify `ephemeral` to `true` as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryStorage:
      local:
        ephemeral: true
  redis:
    imagePullSecretName: redhat-pull-secret
```

## Local Development

Execute the following steps to develop the functionality locally. It is recommended that development be done using a cluster with `cluster-admin` permissions. 

Clone the repository, then resolve all dependencies using `go mod`

```
$ export GO111MODULE=on
$ go mod vendor
```

Using the [operator-sdk](https://github.com/operator-framework/operator-sdk), run the operator locally

```
$ operator-sdk up local --namespace=quay-enterprise
```

### Specifying a Quay Configuration Route

During the development process, you may want to test the provisioning and setup of Quay Enterprise server. By default, the operator will use the internal service to communicate with the configuration pod. However, when running external to the cluster, you will need to specify the ingress location for which the setup process can use.

Specify the `configRoute` as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    configRouteHost: example-quayecosystem-quay-config-quay-enterprise.apps.openshift.example.com
  imagePullSecretName: redhat-pull-secret
```