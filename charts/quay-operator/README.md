# Quay Operator

The Quay Operator manages the lifecycle of the [Quay](https://www.openshift.com/products/quay) container image registry.

## Introduction

This chart installs the Quay Operator to your Kubernetes based cluster.

## Prerequisites

- Kubernetes 1.7+
- A namespace must be available for deploying the chart within. It is recommended that the namespace `quay-enterprise` be used.

## Installation

To install the chart run the following command:

Add the helm repository:

```bash
$ helm repo add quay-operator https://redhat-cop.github.io/quay-operator
```

Install the chart

```bash
$ helm install quay-operator quay-operator/quay-operator
```

## Deleting

To uninstall the chart, uninstall the release

```bash
$ helm uninstall quay-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configuration parameters of the `quay-operator` chart and default values.

|             Parameter            |            Description            |                  Default                  |
|----------------------------------|-----------------------------------|-------------------------------------------|
| `openshift`                 | Indicates whether the chart will be deployed to an OpenShift Container Platform environmment          | `true`        |
| `image.registry`                      | Registry containing the Operator image               | `quay.io`             |
| `image.repository`                     | Repository containing the Operator image              | `redhat-cop/quay-operator`            |
| `image.tag`         | Tag for the Operator image              |  `appVersion` Chart property                         |
| `nameOverride`                | Override the name of the chart                     |                                   |
| `fullnameOverride`         | Overrides the full name of the chart                 |                                 |


Specify any desired parameters using the `--set key=value[,key=value]` argument to `helm install`. For example:

```bash
$ helm install \
    --set image.pullPolicy=IfNotPresent \
    quay-operator .
```

Alternatively, you can use a YAML file that specifies the values while installing the chart. For example:

```bash
$ helm install -f custom_values.yaml .
```