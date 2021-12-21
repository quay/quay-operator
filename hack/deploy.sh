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
#  * the namespace $NAMESPACE exists in the cluster
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
export NAMESPACE=${NAMESPACE:-'quay-operator-e2e-nightly'}
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

yq e -i '.spec.targetNamespaces[0] = env(NAMESPACE)' "${OG_PATH}"
oc apply -n "${NAMESPACE}" -f "${OG_PATH}"

yq e -i '
	.spec.channel = "test" |
	.spec.startingCSV = env(OPERATOR_PKG_NAME) |
	.spec.name = env(OPERATOR_PKG_NAME) |
	.spec.source = env(OPERATOR_PKG_NAME)
' "${SUBSCRIPTION_PATH}"
oc apply -n "${NAMESPACE}" -f "${SUBSCRIPTION_PATH}"

info 'waiting for CSV...'

for n in {1..60}; do
	phase="$(oc get csv "${OPERATOR_PKG_NAME}" -n "${NAMESPACE}" -o jsonpath='{.status.phase}' 2> /dev/null)"
	if [ ! "$phase" = 'Succeeded' ]; then
		if [ "${n}" = "60" ]; then
			error 'timed out waiting'
			info 'csv contents:'
			oc get csv "${OPERATOR_PKG_NAME}" -n "${NAMESPACE}" -o yaml
			exit 1
		fi
		sleep 10
		continue
	fi
	info 'CSV successfully installed'
	break
done

info 'deploy Quay'
oc apply -n "${NAMESPACE}" -f "${QUAY_SAMPLE_PATH}"

# wait for quay deployment to come up. the next checks will fail if the
# resources don't exist.

info 'waiting for quay deployment...'

for n in {1..60}; do
	deploy="$(oc get deploy skynet-quay-app -n "${NAMESPACE}" -o jsonpath='{.metadata.name}' 2> /dev/null)"
	if [ ! "$deploy" = 'skynet-quay-app' ]; then
		if [ "${n}" = "60" ]; then
			error 'timed out waiting'
			exit 1
		fi
		sleep 10
		continue
	fi
	info 'quay deployment created'
	break
done

# the reconcile loop recreates resources a few times before stabilising,
# and the wait command isn't very smart (similar problems exist when waiting
# for rollouts), so we just sleep a bit before waiting.
# see https://github.com/kubernetes/kubectl/issues/1120 for details.
info 'sleeping two minutes before waiting for pods...'
sleep 120

# the order below matters!
# the mirror is the last to come up, so after the dependencies are ready
# we wait for the mirror to be ready, then we check the quay app as it's
# more likely to have stabilized after the mirror is up and running.
info 'waiting for postgres pod...'
oc -n "${NAMESPACE}" wait pods -l=quay-component=postgres --for=condition=Ready --timeout="${WAIT_TIMEOUT}"

info 'waiting for redis pod...'
oc -n "${NAMESPACE}" wait pods -l=quay-component=redis --for=condition=Ready --timeout="${WAIT_TIMEOUT}"

info 'waiting for mirror pod...'
oc -n "${NAMESPACE}" wait pods -l=quay-component=quay-mirror --for=condition=Ready --timeout="${WAIT_TIMEOUT}"

info 'waiting for config-editor pod...'
oc -n "${NAMESPACE}" wait pods -l=quay-component=quay-config-editor --for=condition=Ready --timeout="${WAIT_TIMEOUT}"

info 'waiting for quay pod...'
oc -n "${NAMESPACE}" wait pods -l=quay-component=quay-app --for=condition=Ready --timeout="${WAIT_TIMEOUT}"


# manually check quay's health to ensure it can successfuly connect to its components.
endpoint="$(oc -n "${NAMESPACE}" get quayregistry skynet -o jsonpath='{.status.registryEndpoint}')"
result="$(curl -s -k "${endpoint}"/health/instance)"

if [ "$(echo "${result}" | jq '.status_code')" != "200" ]; then
	error 'quay health check did not return 200'
	info "${result}"
	info 'quayregistry CR yaml:'
	oc -n "${NAMESPACE}" get quayregistry skynet -o yaml
	info 'quay-app pod logs:'
	oc -n "${NAMESPACE}" logs -l=quay-component=quay-app
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.auth')" != "true" ]; then
	error 'quay auth health check was false'
	info "${result}"
	oc -n "${NAMESPACE}" logs -l=quay-component=quay-app
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.database')" != "true" ]; then
	error 'quay database health check was false'
	info "${result}"
	oc -n "${NAMESPACE}" logs -l=quay-component=postgres
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.disk_space')" != "true" ]; then
	error 'quay disk_space health check was false'
	info "${result}"
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.registry_gunicorn')" != "true" ]; then
	error 'quay registry_gunicorn health check was false'
	info "${result}"
	oc -n "${NAMESPACE}" logs -l=quay-component=quay-app
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.service_key')" != "true" ]; then
	error 'quay service_key health check was false'
	info "${result}"
	oc -n "${NAMESPACE}" logs -l=quay-component=quay-app
	exit 1
fi

if [ "$(echo "${result}" | jq '.data.services.web_gunicorn')" != "true" ]; then
	error 'quay web_gunicorn health check was false'
	info "${result}"
	oc -n "${NAMESPACE}" logs -l=quay-component=quay-app
	exit 1
fi

info 'all healthchecks passed'
info 'successfully installed Quay!'
info 'use hack/teardown.sh to remove all created resources'

# shellcheck disable=SC2046
if [ -x $(command -v git >/dev/null 2>&1) ]; then
	git checkout "${CATALOG_PATH}" >/dev/null 2>&1
	git checkout "${SUBSCRIPTION_PATH}" >/dev/null 2>&1
	git checkout "${OG_PATH}" >/dev/null 2>&1
fi
