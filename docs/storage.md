# Registry Storage

Red Hat Quay supports multiple backends for the purpose of image storage and consist of a variety of local and cloud storage options. This page provides an overview on how to configure the Quay Operator to make use of these backends.

## Overview

Storage for Quay can be configured using the `registryBackend` field within the `quay` property in the  `QuayEcosystem` resource which contains an array of backends. The ability to define multiple backends enables replication and high availability of images.

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: backend1
        s3:
        ...
```

The definition of a `registryBackend` is an optional field, and if omitted, _LocalStorage_ will be configured (ephemeral, though the use of a PersistentVolume can be enabled if desired [see _LocalStorage_ configuration below]). 

### Sensitive Values

In many cases, access to storage requires the use of sensitive values. Each backend that requires such configuration can be included in a _Secret_ and defined within the `credentialsSecretName` property of the backend. 

Instead of declaring the registry backend properties within the specific backend, the values can be added to a secret as shown below:

```
oc create secret generic s3-credentials --from-literal=accessKey=<accessKey> --from-literal=secretKey=<secretKey>
```

With the values now present in the secret, the properties explicitly declared in the backend can be removed.


Specific details on the types of properties supported for each backend are found in the registry backend details below.

### Replication

Support is available to replicate the registry storage to multiple backends. To activate storage replication, set the `enableStorageReplication` property to the value of `true`. Individual registry backends can also be configured to be replicated by default by setting the `replicateByDefault` property to the value of `true`. A full configuration demonstrating the replication options available is shown below:

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    enableStorageReplication: true
    registryBackends:
      - name: azure-ussouthcentral
        credentialsSecretName: azure-ussouthcentral-registry
        replicateByDefault: true
        azure:
          azure_container: quay
      - name: azure-seasia
        credentialsSecretName: azure-seasia-registry
        replicateByDefault: true
        azure:
          azure_container: quay
```

_Note:_ Support for replicated storage is not available for the `local` registry backend and will result in an error during the verification phase.


## Registry Storage Backend Types

One or more of the following registry storage backends can be defined to specify the underlying storage for the Quay registry:

### Local Storage

The following is an example for configuring the registry to make use of local storage 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: local
        local:
          storagePath: /opt/quayregistry
```

The following are a comprehensive list of properties for the `local` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |


#### Configuring Persistent Local Storage

By default, Quay uses an ephemeral volume for local storage. In order to avoid data loss, persistent storage is required. To enable the use of a _PersistentVolume_ to store images, specify the `registryStorage` parameter underneath the `quay` property. The following example will cause a _PersistentVolumeClaim_ to be created within the project requesting storage of 10Gi and an _access mode_ of `ReadWriteOnce` (Default value is `ReadWriteMany`)

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryStorage:
      persistentVolumeAccessModes:
        - ReadWriteOnce
      persistentVolumeSize: 10Gi
```

A Storage Class can also be provided using the `persistentVolumeStorageClassName` property


### Amazon Web Services (S3)

The following is an example for configuring the registry to make use of S3 storage on Amazon Web Services 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: s3
        s3:
          accessKey: <accessKey>
          bucketName: <bucketName>
          secretKey: <secretKey>
          host: <host>
```

The following is a comprehensive list of properties for the `s3` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `bucketName` | S3 Bucket | No | Yes |
| `accessKey` | AWS Access Key | Yes | Yes |
| `secretKey` | AWS Secret Key | Yes | Yes |
| `host` | S3 Host | No | No |
| `port` | S3 Port | No | No |



### Microsoft Azure

The following is an example for configuring the registry to make use of Blob storage on the Microsoft Azure platform 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: azure
        azure:
          containerName: <containerName>
          accountName: <accountName>
          accountKey: <accountKey>
```

The following is a comprehensive list of properties for the `azure` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `containerName` | Azure Storage Container | No | Yes |
| `accountName` | Azure Account Name | No | Yes |
| `accountKey` | Azure Account Key | No | Yes |
| `sasToken` | Azure SAS Token | No | No |

### Google Cloud

The following is an example for configuring the registry to make use of Blob storage on the Google Cloud Platform 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: googleCloud
        googleCloud:
          accessKey: <accessKey>
          secretKey: <secretKey>
          bucketName: <bucketName>
```

The following are a comprehensive list of properties for the `googleCloud` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `accessKey` | Cloud Access Key | Yes | Yes |
| `secretKey` | Cloud Secret Key | Yes | Yes |
| `bucketName` | GCS Bucket | No | Yes |

### NooBaa (RHOCS)

The following is an example for configuring the registry to make use of NooBaa storage

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: rhocs
        rhocs:
          hostname: <hostname>
          secure: <secure>
          accessKey: <accessKey>
          secretKey: <secretKey>
          bucketName: <bucketName>
```

The following is a comprehensive list of properties for the `rhocs` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `hostname` | NooBaa Server Hostname | No | Yes |
| `port` | Custom Port | No | No |
| `secure` | Is Secure | No | No |
| `accessKey` | Access Key | Yes | Yes |
| `secretKey` | Secret Key | Yes | Yes |
| `bucketName` | Bucket Name | No | Yes |


### RADOS

The following is an example for configuring the registry to make use of RADOS storage

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: rados
        rados:
          hostname: <hostname>
          secure: <secure>
          accessKey: <accessKey>
          secretKey: <secretKey>
          bucketName: <bucketName>
```

The following are a comprehensive list of properties for the `rados` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `hostname` | Rados Server Hostname | No | Yes |
| `port` | Custom Port | No | No |
| `secure` | Is Secure | No | No |
| `accessKey` | Access Key | Yes | Yes |
| `secretKey` | Secret Key | Yes | Yes |
| `bucketName` | Bucket Name | No | Yes |

### Swift (OpenStack)

The following is an example for configuring the registry to make use of Swift storage

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: swift
        swift:
          authVersion: <authVersion>
          authURL: <authURL>
          container: <container>
          user: <user>
          password: <password>
          caCertPath: <caCertPath>
          osOptions:
            object_storage_url: <object_storage_url>
            user_domain_name: <user_domain_name>
            project_id: <project_id>
```

The following is a comprehensive list of properties for the `swift` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `authVersion` | Swift Auth Version | No | Yes |
| `authURL` | Swift Auth URL | No | Yes |
| `container` | Swift Container Name | No | Yes |
| `user` | Username | Yes | Yes |
| `password` | Key/Password | Yes | Yes |
| `caCertPath` | CA Cert Filename | No | No |
| `tempURLKey` | Temp URL Key | No | No |
| `osOptions` | OS Options | No | No |


### CloudFront (S3)

The following is an example for configuring the registry to make use of S3 storage on Amazon Web Services 

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayEcosystem
metadata:
  name: example-quayecosystem
spec:
  quay:
    registryBackends:
      - name: cloudfrontS3
        cloudfrontS3:
          accessKey: <accessKey>
          bucketName: <bucketName>
          secretKey: <secretKey>
          host: <host>
          distributionDomain: <distributionDomain>
          keyID: <keyID>
          privateKeyFilename: <privateKeyFilename>
```

The following is a comprehensive list of properties for the `cloudfrontS3` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storagePath` | Storage Directory | No | No |
| `bucketName` | S3 Bucket | No | Yes |
| `accessKey` | AWS Access Key | Yes | Yes |
| `secretKey` | AWS Secret Key | Yes | Yes |
| `host` | S3 Host | No | No |
| `port` | S3 Port | No | No |
| `distributionDomain` | CloudFront Distribution Domain Name | No | Yes |
| `keyID` | CloudFront Key ID | No | Yes |
| `privateKeyFilename` | CloudFront Private Key Filename | No | Yes |
