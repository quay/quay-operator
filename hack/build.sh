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
#  * a valid login session to a container registry (unless DRY_RUN=true).
#  * `docker` with buildx
#  * `jq`
#  * `curl`
#  * `opm` and `yq` are auto-installed if missing.
#
# NOTE: this script will modify the following files:
#  - bundle/manifests/quay-operator.clusterserviceversion.yaml
#  - bundle/metadata/annotations.yaml
# if `git` is available it will be used to checkout changes to the above files.
# this means that if you made any changes to them and want them to be persisted,
# make sure to commit them before running this script.
set -ex

export OPERATOR_NAME=${OPERATOR_NAME:-'quay-operator'}
export REGISTRY=${REGISTRY:-'quay.io'}
export NAMESPACE=${NAMESPACE:-'projectquay'}
export TAG=${TAG:-'3.9-unstable'}
export CLAIR_TAG=${CLAIR_TAG:-'4.8.0'}
export BUILDER_QEMU_TAG=${BUILDER_QEMU_TAG:-'main'}
export CHANNEL=${CHANNEL:-'alpha'}
export CSV_PATH=${CSV_PATH:-'bundle/manifests/quay-operator.clusterserviceversion.yaml'}
export ANNOTATIONS_PATH=${ANNOTATIONS_PATH:-'bundle/metadata/annotations.yaml'}
export DRY_RUN=${DRY_RUN:-''}

# derive a semver-compliant version from TAG for use in the CSV.
# opm requires Major.Minor.Patch; TAG may only be Major.Minor (e.g. 3.9-unstable).
BASE=${TAG%%-*}           # strip prerelease: "3.9-unstable" -> "3.9"
PRE=${TAG#"$BASE"}        # extract prerelease: "-unstable" (or empty)
if [[ "$BASE" =~ ^[0-9]+\.[0-9]+$ ]]; then
	VERSION="${BASE}.0${PRE}"
else
	VERSION="${TAG}"
fi
export VERSION

if [ -n "${DRY_RUN}" ]; then
	BUILDX_PUSH="--load"
	BUILDX_PLATFORM="linux/amd64"
else
	BUILDX_PUSH="--push"
	BUILDX_PLATFORM="linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"
fi

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

# install opm and yq if not already on PATH.
function ensure_opm {
	if command -v opm &>/dev/null; then return; fi
	info "installing opm..."
	local version
	version=$(curl -sL https://api.github.com/repos/operator-framework/operator-registry/releases/latest | jq -r .tag_name)
	curl -sLo /usr/local/bin/opm "https://github.com/operator-framework/operator-registry/releases/download/${version}/linux-amd64-opm"
	chmod +x /usr/local/bin/opm
}

function ensure_yq {
	if command -v yq &>/dev/null; then return; fi
	info "installing yq..."
	curl -sLo /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/v4.14.2/yq_linux_amd64"
	chmod +x /usr/local/bin/yq
}

ensure_opm
ensure_yq

function digest() {
	declare -n ret=$2
	IMAGE=$1
	# shellcheck disable=SC2034
	DIGEST=$(docker buildx imagetools inspect "${IMAGE}" --format '{{json .Manifest}}' 2>/dev/null | jq -r .digest)
	if [ -n "${DIGEST}" ] && [ "${DIGEST}" != "null" ]; then
		ret="${IMAGE%%:*}@${DIGEST}"
	else
		ret="${IMAGE}"
	fi
}

docker buildx build ${BUILDX_PUSH} --platform "${BUILDX_PLATFORM}" -t "${REGISTRY}/${NAMESPACE}/quay-operator:${TAG}" .
digest "${REGISTRY}/${NAMESPACE}/quay-operator:${TAG}" OPERATOR_DIGEST

digest "${REGISTRY}/${NAMESPACE}/quay:${TAG}" QUAY_DIGEST
digest "${REGISTRY}/${NAMESPACE}/clair:${CLAIR_TAG}" CLAIR_DIGEST
digest "${REGISTRY}/${NAMESPACE}/quay-builder:${TAG}" BUILDER_DIGEST
digest "${REGISTRY}/${NAMESPACE}/quay-builder-qemu:${BUILDER_QEMU_TAG}" BUILDER_QEMU_DIGEST
digest quay.io/sclorg/postgresql-13-c9s:latest POSTGRES_DIGEST
digest quay.io/sclorg/postgresql-13-c9s:latest POSTGRES_OLD_DIGEST
digest quay.io/sclorg/postgresql-15-c9s:latest POSTGRES_CLAIR_DIGEST
digest quay.io/sclorg/postgresql-13-c9s:latest POSTGRES_CLAIR_OLD_DIGEST
digest docker.io/library/redis:7.0 REDIS_DIGEST

# need exporting so that yq can see them
export OPERATOR_DIGEST
export QUAY_DIGEST
export CLAIR_DIGEST
export BUILDER_DIGEST
export BUILDER_QEMU_DIGEST
export POSTGRES_DIGEST
export POSTGRES_OLD_DIGEST
export POSTGRES_CLAIR_DIGEST
export POSTGRES_CLAIR_OLD_DIGEST
export REDIS_DIGEST


# prepare operator files, then build and push operator bundle and catalog
# index images.

yq eval -i '
	.metadata.name = ("quay-operator.v" + strenv(VERSION)) |
	.metadata.annotations.quay-version = strenv(TAG) |
	.metadata.annotations.containerImage = strenv(OPERATOR_DIGEST) |
	.metadata.annotations["olm.skipRange"] = (">=3.6.x <" + strenv(VERSION)) |
	del(.spec.replaces) |
	.spec.version = strenv(VERSION) |
	.spec.install.spec.deployments[0].name = strenv(OPERATOR_NAME) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = strenv(OPERATOR_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_QUAY") .value = strenv(QUAY_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_CLAIR") .value = strenv(CLAIR_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_BUILDER") .value = strenv(BUILDER_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_BUILDER_QEMU") .value = strenv(BUILDER_QEMU_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES") .value = strenv(POSTGRES_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_POSTGRES_PREVIOUS") .value = strenv(POSTGRES_OLD_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES") .value = strenv(POSTGRES_CLAIR_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES_PREVIOUS") .value = strenv(POSTGRES_CLAIR_OLD_DIGEST) |
	.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] |= select(.name == "RELATED_IMAGE_COMPONENT_REDIS") .value = strenv(REDIS_DIGEST)
	' "${CSV_PATH}"

yq eval -i '
	.annotations."operators.operatorframework.io.bundle.channel.default.v1" = strenv(CHANNEL) |
	.annotations."operators.operatorframework.io.bundle.channels.v1" = strenv(CHANNEL) |
	.annotations."operators.operatorframework.io.bundle.package.v1" = "project-quay"
	' "${ANNOTATIONS_PATH}"

docker buildx build ${BUILDX_PUSH} -f ./bundle/Dockerfile --platform "${BUILDX_PLATFORM}" -t "${REGISTRY}/${NAMESPACE}/quay-operator-bundle:${TAG}" ./bundle

# build file-based catalog (FBC) index image.
# in dry-run mode, render from local bundle directory; otherwise from the pushed bundle image.
if [ -n "${DRY_RUN}" ]; then
	BUNDLE_REF="bundle/"
else
	BUNDLE_REF="${REGISTRY}/${NAMESPACE}/quay-operator-bundle:${TAG}"
fi

mkdir -p catalog/project-quay
opm render "${BUNDLE_REF}" --output=yaml > catalog/project-quay/catalog.yaml

# opm render from a local directory does not produce an olm.package entry;
# add one if missing so opm validate succeeds in both dry-run and production.
if ! grep -q 'schema: olm.package' catalog/project-quay/catalog.yaml; then
cat >> catalog/project-quay/catalog.yaml <<PKGEOF
---
schema: olm.package
name: project-quay
defaultChannel: ${CHANNEL}
PKGEOF
fi

cat >> catalog/project-quay/catalog.yaml <<CHEOF
---
schema: olm.channel
package: project-quay
name: ${CHANNEL}
entries:
  - name: quay-operator.v${VERSION}
CHEOF

opm validate catalog
opm generate dockerfile catalog

docker buildx build ${BUILDX_PUSH} --platform "${BUILDX_PLATFORM}" \
	-f catalog.Dockerfile -t "${REGISTRY}/${NAMESPACE}/quay-operator-index:${TAG}" .
