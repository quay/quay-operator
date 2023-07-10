#!/bin/sh -e

# Single quotes for expressions are used intentionally to delegate variable
# substitution to yq.
# shellcheck disable=SC2016

yq eval -i '
    .metadata.annotations.createdAt = (now | tz("UTC") | format_datetime("2006-01-02 15:04 UTC")) |
    .metadata.annotations["olm.skipRange"] = (">=3.6.x <${RELEASE}" | envsubst) |
    .metadata.annotations["quay-version"] = strenv(RELEASE) |
    .metadata.annotations.containerImage = ("quay.io/projectquay/quay-operator:v${RELEASE}" | envsubst) |
    .metadata.name = ("quay-operator.v${RELEASE}" | envsubst) |
    .spec.install.spec.deployments[0].spec.template.spec.containers[0].image = ("quay.io/projectquay/quay-operator:v${RELEASE}" | envsubst) |
    .spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= (
        select(.name == "RELATED_IMAGE_COMPONENT_BUILDER").value = ("quay.io/projectquay/quay-builder:v${RELEASE}" | envsubst) |
        select(.name == "RELATED_IMAGE_COMPONENT_CLAIR").value = ("quay.io/projectquay/clair:v${CLAIR_RELEASE}" | envsubst) |
        select(.name == "RELATED_IMAGE_COMPONENT_QUAY").value = ("quay.io/projectquay/quay:v${RELEASE}" | envsubst)
    ) |
    .spec.version = strenv(RELEASE) |
    .spec.replaces = strenv(REPLACES)
' bundle/manifests/quay-operator.clusterserviceversion.yaml

yq eval -i '
    .annotations["operators.operatorframework.io.bundle.channel.default.v1"] = strenv(DEFAULT_CHANNEL) |
    .annotations["operators.operatorframework.io.bundle.channels.v1"] = strenv(CHANNEL) |
    .annotations["operators.operatorframework.io.bundle.package.v1"] = "project-quay"
' bundle/metadata/annotations.yaml
