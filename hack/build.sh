#!/usr/bin/env bash
# builds the operator and its OLM catalog index and pushes it to quay.io.
#
# by default, the built catalog index is tagged with
# `quay.io/projectquay/quay-operator-index:3.6-unstable`. you can override the
# tag alone by exporting TAG before executing this script.
#
# To push to your own registry, override the REGISTRY and NAMESPACE env vars,
# i.e:
#   $ REGISTRY=quay.io NAMESPACE=yourusername ./hack/build.sh
#
# REQUIREMENTS:
#  * a valid login session to a container registry.
#  * `docker`
#  * `yq`
#  * `opm`
#
# NOTE: this script will modify the following files:
#  - bundle/manifests/quay-operator.clusterserviceversion.yaml
#  - bundle/metadata/annotations.yaml
# if `git` is available it will be used to checkout changes to the above files.
# this means that if you made any changes to them and want them to be persisted,
# make sure to commit them before running this script.
set -e

export OPERATOR_NAME='quay-operator-test'
export REGISTRY=${REGISTRY:-'quay.io'}
export NAMESPACE=${NAMESPACE:-'projectquay'}
export TAG=${TAG:-'3.6-unstable'}
export CSV_PATH=${CSV_PATH:-'bundle/manifests/quay-operator.clusterserviceversion.yaml'}
export ANNOTATIONS_PATH=${ANNOTATIONS_PATH:-'bundle/metadata/annotations.yaml'}

function cleanup {
	# shellcheck disable=SC2046
	if [ -x $(command -v git >/dev/null 2>&1) ]; then
		git checkout "${CSV_PATH}" >/dev/null 2>&1
		git checkout "${ANNOTATIONS_PATH}" >/dev/null 2>&1
	fi
}

trap cleanup EXIT

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

# prints pre-formatted error output.
function error {
	>&2 echo "ERROR $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

function digest() {
	declare -n ret=$2
	IMAGE=$1
	docker pull "${IMAGE}"
	# shellcheck disable=SC2034
	ret=$(docker inspect --format='{{index .RepoDigests 0}}' "${IMAGE}")
}

docker build -t "${REGISTRY}/${NAMESPACE}/quay-operator:${TAG}" .
docker push "${REGISTRY}/${NAMESPACE}/quay-operator:${TAG}"
digest "${REGISTRY}/${NAMESPACE}/quay-operator:${TAG}" OPERATOR_DIGEST

digest "${REGISTRY}/${NAMESPACE}/quay:${TAG}" QUAY_DIGEST
digest "${REGISTRY}/${NAMESPACE}/clair:nightly" CLAIR_DIGEST
digest "${REGISTRY}/${NAMESPACE}/quay-builder:${TAG}" BUILDER_DIGEST
digest "${REGISTRY}/${NAMESPACE}/quay-builder-qemu:main" BUILDER_QEMU_DIGEST
# shellcheck disable=SC2034
POSTGRES_DIGEST='centos/postgresql-13-centos7@sha256:71b24684d64da46f960682cc4216222a7e4ed8b1a31dd5a865b3e71afdea20d2'
# shellcheck disable=SC2034
POSTGRES_OLD_DIGEST='centos/postgresql-10-centos7@sha256:de1560cb35e5ec643e7b3a772ebaac8e3a7a2a8e8271d9e91ff023539b4dfb33'

# need exporting so that yq can see them
export OPERATOR_DIGEST
export QUAY_DIGEST
export CLAIR_DIGEST
export BUILDER_DIGEST
export BUILDER_QEMU_DIGEST
export POSTGRES_DIGEST
export REDIS_DIGEST


# prepare operator files, then build and push operator bundle and catalog
# index images.

yq eval -i '
	.metadata.name = strenv(OPERATOR_NAME) |
	.metadata.annotations.quay-version = strenv(TAG) |
	.metadata.annotations.containerImage = strenv(OPERATOR_DIGEST) |
	del(.spec.replaces) |
	.spec.install.spec.deployments[0].name = strenv(OPERATOR_NAME) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = strenv(OPERATOR_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_QUAY") .value = strenv(QUAY_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_CLAIR") .value = strenv(CLAIR_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_BUILDER") .value = strenv(BUILDER_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_BUILDER_QEMU") .value = strenv(BUILDER_QEMU_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES") .value = strenv(POSTGRES_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES_PREVIOUS") .value = strenv(POSTGRES_OLD_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_REDIS") .value = strenv(REDIS_DIGEST)
	' "${CSV_PATH}"

yq eval -i '
	.annotations."operators.operatorframework.io.bundle.channel.default.v1" = "test" |
	.annotations."operators.operatorframework.io.bundle.channels.v1" = "test"
	' "${ANNOTATIONS_PATH}"

docker build -f ./bundle/Dockerfile -t "${REGISTRY}/${NAMESPACE}/quay-operator-bundle:${TAG}" ./bundle
docker push "${REGISTRY}/${NAMESPACE}/quay-operator-bundle:${TAG}"
digest "${REGISTRY}/${NAMESPACE}/quay-operator-bundle:${TAG}" BUNDLE_DIGEST

opm index add --build-tool docker --bundles "${BUNDLE_DIGEST}" --tag "${REGISTRY}/${NAMESPACE}/quay-operator-index:${TAG}"
docker push "${REGISTRY}/${NAMESPACE}/quay-operator-index:${TAG}"
