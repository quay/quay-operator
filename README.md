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

Quay recommends that it by deployed in a namespace called `quay-enterprise`, however support is available for deploying the operator to a namespace of your choosing. When choosing a namespace other than `quay-enterprise`, the _namespace_ field in the [deploy/cluster_role_binding.yaml](deploy/cluster_role_binding.yaml) must be updated with the new namespace otherwise permission issues will occur.

The steps described below assume the namespace that will be utilized is called `quay-enterprise`.

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

Create a pull secret to retrieve Quay images from quay.io. If unsure what to use for the pull secret see [Accessing Red Hat Quay (formerly Quay Enterprise) without a CoreOS login](https://access.redhat.com/solutions/3533201).

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

Note: It is important to note that persistent storage for the database will **only** be provisioned if the `volumeSize` property is specified when provisioned by the operator.

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

### Registry Backends

Quay supports multiple storage backends (configured as an array). The quay operator supports aiding in the facilitation of certain storage backends. The following backends are currently supported:

* Local
* S3

#### Local Storage

The following is an example of how to define a local backend with a customized location for which images will be stored:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryBackends:
      - name: local
        local:
          storage_path: /opt/quayregistry
```


##### Configuring Persistent Local Storage

By default, Quay uses an ephemeral volume for local storage. In order to avoid data loss, persistent storage is required. To enable the use of a _PersistentVolume_ to store images, specify the `registryStorage` parameter underneath the `quay` property. The following example will cause a _PersistentVolumeClaim_ to be created within the project requesting storage of 10Gi and an _access mode_ of `ReadWriteOnce` (Default value is `ReadWriteMany`)

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryStorage:
      persistentVolumeAccessMode:
        - ReadWriteOnce
      persistentVolumeSize: 10Gi
```

A Storage Class can also be provided using the `persistentVolumeStorageClassName` property

### S3 Storage

S3 Storage, for example from AWS, is supported using the `s3` registry backend type. The following is an example of how an S3 bucket in AWS can be used as the registry storage:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryStorage:
      - name: s3
        s3:
          s3_bucket: quay
          s3_access_key: <s3_access_key>
          s3_secret_key: <s3_secret_key>
          host: s3.us-east-2.amazonaws.com
```

#### S3 Credentials Secret

To prevent displaying sensitive values as plaintext, the secret key and access key can be added to a secret which in turn can be reference within the `QuayEcosystem` resource. 

```
oc create secret generic s3-credentials --from-literal=s3_access_key=<s3_access_key> --from-literal=s3_secret_key=<s3_secret_key>
```

Remove the `s3_access_key` and `s3_secret_key` properties if specified and reference the secret using the 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    registryStorage:
      - name: s3
        s3:
          s3_bucket: quay
          credentialsSecretName: s3-credentials
          host: s3.us-east-2.amazonaws.com
```

### Storage Replication

Data can be replicated between multiple registry backends and can be enabled by setting the `enableStorageReplication` to `true` as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    enableStorageReplication: true
    registryStorage:
      - name: s3
        s3:
          s3_bucket: quay
          credentialsSecretName: s3-credentials
          host: s3.us-east-2.amazonaws.com
          replicateByDefault: true
      - name: s3-backup
        s3:
          s3_bucket: quay-backup
          credentialsSecretName: s3-credentials
          host: s3.us-east-2.amazonaws.com
          replicateByDefault: true
```

Each backend that supports replication contains a `replicateByDefault` field that when set to `true` will be distributed to other storage engines.

_Note:_ Support for replicated storage is not available for the `local` registry backend and will result in an error during the verification phase.

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

### Specifying the Quay Route

Quay makes use of an OpenShift route to enable ingress. The hostname for this route is automatically generated as per the configuration of the OpenShift cluster. Alternatively, the hostname for this route can be explicitly specified using the `routeHost` property under the _quay_ field as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    routeHost: example-quayecosystem-quay-quay-enterprise.apps.openshift.example.com
    imagePullSecretName: redhat-pull-secret
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

### Configuration Deployment After Initial Setup

In order to conserve resources, the configuration deployment of Quay is removed after the initial setup. In certain cases, there may be a need to further configure the quay environment. To specify that the configuration deployment should be retained, the `keepConfigDeployment` property within the _Quay_ object can can be set as `true` as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    keepConfigDeployment: true
```

### Redis Password

By default, the operator managed Redis instance is deployed without a password. A password can be specified by creating a secret containing the password in the key _password_. The following command can be used to create the secret:

```
oc create secret generic <secret_name> --from-literal=password=<password>
```

The secret can then be specified within the _redis_ section using the `` as shown below:


```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  redis:
    credentialsSecretName: <secret_name>
    imagePullSecretName: redhat-pull-secret
```

### Clair

[Clair](https://github.com/coreos/clair) is a vulnerability assessment tool for application container. Support is available to automatically provision and configure both Clair and the integration wtih Quay. A property called `clair` can be specified in the `QuayEcosystem` object along with `enabled: true` within this field in order to deploy Clair. An example is shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
  clair:
    enabled: true
    imagePullSecretName: redhat-pull-secret
```

#### Update Interval

Clair routinely queries CVE databases in order to build its own internal database. By default, this value is set at 500m. You can modify the time interval between checks by setting the `updateInterval` property as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
  clair:
    enabled: true
    imagePullSecretName: redhat-pull-secret
    updateInterval: "60m"
```

The above configuration would have Clair update every 60 minutes.

## Common Attributes

Each of the following components expose a set of similar properties that can be specified in order to customize the runtime execution:

* Quay
* Quay Configuration
* Quay PostgreSQL
* Redis
* Clair
* Clair PostgreSQL

### Image Pull Secret

As referenced in prior sections, an Image Pull Secret can specify the name of the secret containing credentials to an image from a protected registry using the property `imagePullSecret`.

### Compute Resources

[Compute Resources](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container) such as memory and CPU can be specified in the same form as any other value in a `PodTemplate`. CPU and Memory values for _Requests_ and _Limits_ can be specified under a property called `resources`.

_Note:_ In the case of the QuayConfiguration deployment, `configResources` is the property which should be referenced underneath the `Quay` property.

The following is an example of how compute resources can be specified:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    resources:
      requests:
        memory: 512Mi
```

### Probes

[Readiness and Liveness Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/) can be specified in the same form as any other value in a `PodTemplate`. 

The following is how a _readinessProbe_ and _livenessProbe_ can be specified:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    livenessProbe:
      initialDelaySeconds: 120
      httpGet:
        path: /health/instance
        port: 8443
        scheme: HTTPS
    readinessProbe:
      initialDelaySeconds: 10
      httpGet:
        path: /health/instance
        port: 8443
        scheme: HTTPS
```

_Note_: If a value for either property is not specified, an opinionated default value is applied.

### Node Selector

It may be desired that components of the `QuayEcosystem` may need to be deployed to only a subset of available nodes in a Kubernetes cluster. This functionality can be set on each of the resources using the `nodeSelector` property as show below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    nodeSelector:
      node-role.kubernetes.io/infra=true
```

### Deployment Strategy

Each of the core components consist of Kubernetes `Deployments`. This resource supports the method in which new versions are released. This operator supports making use of the `RollingUpdate` and `Recreate` strategies. Either value can be defined by using the `deploymentStrategy` property on the desired resource as shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    deploymentStrategy: RollingUpdate
```

_Note: The absence of a defined value will make use of the `RollingUpdate` strategy_

### Environment Variables

In addition to environment variables that are automatically configured by the operator, users can define their own set of environment variables in order to customize the managed resources. Each core component includes a property called `envVars` where environment variables can be defined. An example is shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    imagePullSecretName: redhat-pull-secret
    envVars:
      - name: FOO
        value: bar
```

_Note_: Environment variables for the Quay configuration pod can be managed by specifying the `configEnvVars` property on the `quay` resource

_Caution:_ User defined environment variables are given precedence over those managed by the operator. Undesirable results may occur if conflicting keys are used.

## Troubleshooting

To resolve issues running, configuring and utilzing the operator, the following steps may be utilized:

### Errors during initial setup

The _QuayEcosystem_ custom resource will attempt to provide the progress of the status of the deployment and configuration of Quay. Additional information related to any errors in the setup process can be found by viewing the log messages of the _config_ pod as shown below:

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
