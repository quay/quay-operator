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
    superuserCredentialsSecretName: <secret_name>
    imagePullSecretName: redhat-pull-secret
```

#### Quay Configuration

A dedicated deployment of Quay Enterprise is used to manage the configuration of Quay. Access to the configuration interface is secured and requires authentication in order for access. By default, the following values are used:

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
```

_Note: The superuser password must be at least eight characters._

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
```

#### Using an Existing PostgreSQL Instance

Instead of having the operator deploy an instance of PostgreSQL in the project, an existing instance can be leveraged by specifying the location in the `server` field along with the credentials for access as described in the previous section. The following is an example of how to specify connecting to a remote PostgreSQL instance

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
      server: postgresql.databases.example.com
```

### Registry Storage

Quay supports multiple storage backends. The quay operator supports aiding in the facilitation of certain storage backends. The following backends are currently supported:

* Local

#### Configuring Local Storage

Local storage references a local directory within the Quay pod for which image metadata is stored. The configuration is specified by using the `registryStorage` parameters underneath the `quay` property. By default, Quay operates with no persistent storage. In order to avoid data loss, persistent storage is required. The following example will cause a _PersistentVolumeClaim_ to be created within the project requesting storage of 10Gi and an _access mode_ of `ReadWriteOnce` (Default value is `ReadWriteMany`)

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
```

A Storage Class can also be provided using the `persistentVolumeStorageClassName` property

### Skipping Automated Setup

The operator by default is configured to complete the automated setup process for Quay. This can be bypassed by setting the `skipSetup` field to `true` as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    skipSetup: true
```

### SSL Certificates

Quay, as a secure registry, makes use of SSL certificates to secure communication between the various components within the ecosystem. Transport to the Quay user interface and container registry is secured via SSL certificates. These certificates are generated at startup with the OpenShift route being configured with a TLS termination type of _Passthrough_.

#### User Provided Certificates

SSL certificates can be provided and used instead of having the operator generate certificates. Certificates can be provided in a secret which is then referenced in the _QuayEcosystem_ custom resource. 

The secret containing custom certificates must define the following keys:

* `ssl.cert` -  All of the certificates (root, intermediate, certificate) concatinated into a single file 
* `ssl.key` - Private key as for the SSL certificate

Create a secret containing the certificate and private key

```
oc create secret generic custom-quay-ssl --from-file=ssl.key=<ssl_private_key> --from-file=ssl.cert=<ssl_certificate>
```

The secret containing the certificates are referenced using the `sslCertificatesSecretName` proprety as shown below

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    sslCertificatesSecretName: custom-quay-ssl
```

### Configuration Deployment Removal

In order to allow for continued access to configure Quay, the configuration deployment of Quay remains active after the initial deployment and configuration of Quay. To reclaim the configuration deployment resources, the `keepConfigDeployment` can be set as `false` as shown below

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    keepConfigDeployment: false
```

## Troubleshooting

To resolve issues running, configuring and utilzing the operator, the following steps may be utilized:

### Errors during initial setup

The _QuayEcosystem_custom resource will attempt to provide the progress of the status of the deployment and configuration of Quay. Additional information related to any errors in the setup process can be found by viewing the log messages of the _config_ pod as shown below:

```
oc logs $(oc get pods -l=quay-enterprise-component=config -o name)
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
