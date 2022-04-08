#!/bin/bash
# deploys the operator to a cluster from a published version of its catalog
# index.
#
# by default, the deployed operator image is
# `quay.io/projectquay/quay-operator-index:3.6-unstable`. you can override the
# tag alone by exporting TAG before exeucting this script, or override the
# whole thing by setting CATALOG_IMAGE.
#
# REQUIREMENTS:
#  * a valid login session to an OCP cluster, with cluster admin privileges
#  * noobaa backing storage (TODO: elaborate)
#  * `curl`
#  * `docker`
#  * `yq`
#  * `jq`
#  * `oc`
#
# NOTE: this script will modify the following files:
#  - bundle/quay-operator.catalogsource.yaml
#  - bundle/quay-operator.operator-group.yaml
#  - bundle/quay-operator.subscription.yaml
# if `git` is available it will be used to checkout changes to the above files.
# this means that if you made any changes to them and want them to be persisted,
# make sure to commit them before running this script.

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

# prints pre-formatted error output.
function error {
	>&2 echo "ERROR $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

export TAG=${TAG:-'3.6-unstable'}
export CATALOG_PATH=${CATALOG_PATH:-'./bundle/quay-operator.catalogsource.yaml'}
export CATALOG_IMAGE=${CATALOG_IMAGE:-"quay.io/projectquay/quay-operator-index:${TAG}"}
export OG_PATH=${OG_PATH:-'./bundle/quay-operator.operatorgroup.yaml'}
export SUBSCRIPTION_PATH=${SUBSCRIPTION_PATH:-'./bundle/quay-operator.subscription.yaml'}
export QUAY_SAMPLE_PATH=${QUAY_SAMPLE_PATH:-'./config/samples/managed.quayregistry.yaml'}
export OPERATOR_PKG_NAME=${OPERATOR_PKG_NAME:-'quay-operator-test'}
export WAIT_TIMEOUT=${WAIT_TIMEOUT:-'20m'}

info 'calculating catalog index image digest'

docker pull "${CATALOG_IMAGE}"
CAT_IMG_DIGEST="$(docker inspect --format='{{index .RepoDigests 0}}' "${CATALOG_IMAGE}")"
export CAT_IMG_DIGEST

info 'setting up catalog source'

yq e -i '
	.spec.image = env(CAT_IMG_DIGEST) |
	.metadata.name = env(OPERATOR_PKG_NAME)
' "${CATALOG_PATH}"

oc apply -n openshift-marketplace -f "${CATALOG_PATH}"
info 'waiting for catalog source to become available...'
for n in {1..60}; do
	pkgmanifest="$(oc get packagemanifest "${OPERATOR_PKG_NAME}" -n openshift-marketplace -o jsonpath='{.status.catalogSource}' 2> /dev/null)"
	if [ ! "$pkgmanifest" = "${OPERATOR_PKG_NAME}" ]; then
		if [ "${n}" = "60" ]; then
			error 'timed out waiting'
			info 'catalog source pod yaml:'
			oc -n openshift-marketplace get pods -l=olm.catalogSource="${OPERATOR_PKG_NAME}"
			exit 1
		fi
		sleep 10
		continue
	fi
	info 'catalog source installed and available'
	break
done

info 'installing the operator'

oc apply -f "${OG_PATH}"

yq e -i '
	.spec.channel = "test" |
	.spec.startingCSV = env(OPERATOR_PKG_NAME) |
	.spec.name = env(OPERATOR_PKG_NAME) |
	.spec.source = env(OPERATOR_PKG_NAME)
' "${SUBSCRIPTION_PATH}"
oc apply -f "${SUBSCRIPTION_PATH}"

info 'waiting for CSV...'

for n in {1..60}; do
	phase="$(oc get csv "${OPERATOR_PKG_NAME}" -o jsonpath='{.status.phase}' 2> /dev/null)"
	if [ ! "$phase" = 'Succeeded' ]; then
		if [ "${n}" = "60" ]; then
			error 'timed out waiting'
			info 'csv contents:'
			oc get csv "${OPERATOR_PKG_NAME}" -o yaml
			exit 1
		fi
		sleep 10
		continue
	fi
	info 'CSV successfully installed'
	break
done

# shellcheck disable=SC2046
if [ -x $(command -v git >/dev/null 2>&1) ]; then
	git checkout "${CATALOG_PATH}" >/dev/null 2>&1
	git checkout "${SUBSCRIPTION_PATH}" >/dev/null 2>&1
	git checkout "${OG_PATH}" >/dev/null 2>&1
fi
