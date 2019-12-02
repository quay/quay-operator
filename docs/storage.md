# Registry Storage

Red Hat Quay supports multiple backends for the purpose of image storage and consist of a variety of local and cloud storage options. This page provides an overview how to configure the Quay Operator to make use of these backends.

## Overview

Storage for Quay can be configured using the `registryBackend` field within the `quay` property in the  `QuayEcosystem` resource which contain an array of backends. The ability to define multiple backends enables replication and high availability of images.

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
oc create secret generic s3-credentials --from-literal=s3_access_key=<s3_access_key> --from-literal=s3_secret_key=<s3_secret_key>
```

With the values now present in the secret, the properties explicitly declared in the backend can be removed.


Specific details on the types of properties supported for each backend are found in the registry backend details below.

### Replication

Support is available to replicate the registry storage to multiple backends. To activate storage replication, set the `enableStorageReplication` property to the value of `true`. Individual registry backends can also be configured to be replicated by default by setting the `replicateByDefault` property to tha value of `true`. A full configuration demonstrating the replication options available is shown below:

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
        azure:
          azure_container: quay
          replicateByDefault: true
      - name: azure-seasia
        credentialsSecretName: azure-seasia-registry
        azure:
          azure_container: quay
          replicateByDefault: true
```

_Note:_ Support for replicated storage is not available for the `local` registry backend and will result in an error during the verification phase.


## Registry Storage Backend Types

One or more of the following registry storage backends can be defined to specify the underlying storage for the Quay registry:

### Local Storage

The following is an example for configuring the registry to make use of Local storage 

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
          storage_path: /opt/quayregistry
```

The following are a comprehensive list of properties for the `local` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |


#### Configuring Persistent Local Storage

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
          s3_access_key: <s3_access_key>
          s3_bucket: <s3_bucket>
          s3_secret_key: <s3_scret_key>
          host: <host>
```

The following are a comprehensive list of properties for the `s3` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `s3_bucket` | S3 Bucket | No | Yes |
| `s3_access_key` | AWS Access Key | Yes | Yes |
| `s3_secret_key` | AWS Secret Key | Yes | Yes |
| `host` | S3 Host | No | No |
| `port` | S3 Port | No | No |



### Microsoftt Azure

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
          azure_container: <azure_container>
          azure_account_name: <azure_account_name>
          azure_account_key: <azure_account_key>
```

The following are a comprehensive list of properties for the `azure` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `azure_container` | Azure Storage Container | No | Yes |
| `azure_account_name` | Azure Account Name | No | Yes |
| `azure_account_key` | Azure Account Key | No | Yes |
| `sas_token` | Azure SAS Token | No | No |

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
      - name: googlecloud
        googlecloud:
          azure_container: <azure_container>
          azure_account_name: <azure_account_name>
          azure_account_key: <azure_account_key>
```

The following are a comprehensive list of properties for the `azure` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `access_key` | Cloud Access Key | Yes | Yes |
| `secret_key` | Cloud Secret Key | Yes | Yes |
| `bucket_name` | GCS Bucket | No | Yes |

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
          is_sucure: <is_secure>
          access_key: <access_key>
          secret_key: <secret_key>
          bucket_name: <bucket_name>
```

The following are a comprehensive list of properties for the `rhocs` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `hostname` | NooBaa Server Hostname | No | Yes |
| `port` | Custom Port | No | No |
| `is_secure` | Is Secure | No | No |
| `access_key` | Access Key | Yes | Yes |
| `secret_key` | Secret Key | Yes | Yes |
| `bucket_name` | Bucket Name | No | Yes |


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
        rhocs:
          hostname: <hostname>
          is_sucure: <is_secure>
          access_key: <access_key>
          secret_key: <secret_key>
          bucket_name: <bucket_name>
```

The following are a comprehensive list of properties for the `rados` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `hostname` | Rados Server Hostname | No | Yes |
| `port` | Custom Port | No | No |
| `is_secure` | Is Secure | No | No |
| `access_key` | Access Key | Yes | Yes |
| `secret_key` | Secret Key | Yes | Yes |
| `bucket_name` | Bucket Name | No | Yes |

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
        rhocs:
          auth_version: <auth_version>
          auth_url: <auth_url>
          swift_container: <swift_container>
          swift_user: <swift_user>
          swift_password: <swift_password>
          ca_cert_path: <ca_cert_path>
          os_options:
            object_storage_url: <object_storage_url>
            user_domain_name: <user_domain_name>
            project_id: <project_id>
```

The following are a comprehensive list of properties for the `rados` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `auth_version` | Swift Auth Version | No | Yes |
| `auth_url` | Swift Auth URL | No | Yes |
| `swift_container` | Swift Container Name | No | Yes |
| `swift_user` | Username | Yes | Yes |
| `swift_password` | Key/Password | Yes | Yes |
| `ca_cert_path` | CA Cert Filename | No | No |
| `temp_url_key` | Temp URL Key | No | No |
| `os_options` | OS Options | No | No |


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
      - name: s3
        s3:
          s3_access_key: <s3_access_key>
          s3_bucket: <s3_bucket>
          s3_secret_key: <s3_scret_key>
          host: <host>
          cloudfront_distribution_domain: <cloudfront_distribution_domain>
          cloudfront_key_id: <cloudfront_key_id>
          cloudfront_privatekey_filename: <cloudfront_privatekey_filename>
```

The following are a comprehensive list of properties for the `cloudfronts3` registry backend:

| Property | Description | Credential Secret Supported | Required |
| -------- | ----------- | --------------------------- | -------- |
| `storage_path` | Storage Directory | No | No |
| `s3_bucket` | S3 Bucket | No | Yes |
| `s3_access_key` | AWS Access Key | Yes | Yes |
| `s3_secret_key` | AWS Secret Key | Yes | Yes |
| `host` | S3 Host | No | No |
| `port` | S3 Port | No | No |
| `cloudfront_distribution_domain` | CloudFront Distribution Domain Name | No | Yes |
| `cloudfront_key_id` | CloudFront Key ID | No | Yes |
| `cloudfront_privatekey_filename` | CloudFront Private Key | No | Yes |
