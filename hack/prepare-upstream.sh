#!/bin/sh -e

# Single quotes for expressions are used intentionally to delegate variable
# substitution to yq.
# shellcheck disable=SC2016

current_image() {
    export YQ_COMPONENT="$1"
    yq eval '
        .spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |
            select(.name == ("RELATED_IMAGE_COMPONENT_${YQ_COMPONENT}" | envsubst)).value
    ' ./bundle/manifests/quay-operator.clusterserviceversion.yaml
}

digest() {
    local image
    image=$(current_image "$1")
    docker pull "$image" >/dev/null
    docker inspect --format='{{index .RepoDigests 0}}' "$image"
}

POSTGRES_DIGEST=$(digest POSTGRES)
POSTGRES_PREVIOUS_DIGEST=$(digest POSTGRES_PREVIOUS)
POSTGRES_CLAIR_DIGEST=$(digest POSTGRES_CLAIR)
POSTGRES_CLAIR_PREVIOUS_DIGEST=$(digest POSTGRES_CLAIR_PREVIOUS)
REDIS_DIGEST=$(digest REDIS)

# export variables for yq
export POSTGRES_DIGEST
export POSTGRES_PREVIOUS_DIGEST
export POSTGRES_CLAIR_DIGEST
export POSTGRES_CLAIR_PREVIOUS_DIGEST
export REDIS_DIGEST

yq eval -i '
    .metadata.annotations.createdAt = (now | tz("UTC")) |
    .metadata.annotations["olm.skipRange"] = (">=3.6.x <${RELEASE}" | envsubst) |
    .metadata.annotations["quay-version"] = strenv(RELEASE) |
    .metadata.annotations.containerImage = ("quay.io/projectquay/quay-operator:${RELEASE}" | envsubst) |
    .metadata.name = ("quay-operator.v${RELEASE}" | envsubst) |
    .spec.install.spec.deployments[0].spec.template.spec.containers[0].image = ("quay.io/projectquay/quay-operator:${RELEASE}" | envsubst) |
    .spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= (
        select(.name == "RELATED_IMAGE_COMPONENT_BUILDER").value = ("quay.io/projectquay/quay-builder:${RELEASE}" | envsubst) |
        select(.name == "RELATED_IMAGE_COMPONENT_CLAIR").value = ("quay.io/projectquay/clair:${CLAIR_RELEASE}" | envsubst) |
        select(.name == "RELATED_IMAGE_COMPONENT_QUAY").value = ("quay.io/projectquay/quay:${RELEASE}" | envsubst) |
        select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES").value = strenv(POSTGRES_DIGEST) |
        select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES_PREVIOUS").value = strenv(POSTGRES_PREVIOUS_DIGEST) |
        select(.name == "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES").value = strenv(POSTGRES_CLAIR_DIGEST) |
        select(.name == "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES_PREVIOUS").value = strenv(POSTGRES_CLAIR_PREVIOUS_DIGEST) |
        select(.name == "RELATED_IMAGE_COMPONENT_REDIS").value = strenv(REDIS_DIGEST)
    ) |
    .spec.version = strenv(RELEASE) |
    .spec.replaces = strenv(REPLACES)
' ./bundle/manifests/quay-operator.clusterserviceversion.yaml

yq eval -i '
    .annotations["operators.operatorframework.io.bundle.channel.default.v1"] = strenv(DEFAULT_CHANNEL) |
    .annotations["operators.operatorframework.io.bundle.channels.v1"] = strenv(CHANNEL) |
    .annotations["operators.operatorframework.io.bundle.package.v1"] = "project-quay"
' ./bundle/metadata/annotations.yaml
