#!/bin/bash
# teardown the operator deployed by hack/deploy.sh
#
# REQUIREMENTS:
#  * a valid login session to an OCP cluster, with cluster admin privileges
#  * `yq` cmd line tool
#  * `oc` cmd line tool

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

export OPERATOR_PKG_NAME=${OPERATOR_PKG_NAME:-'quay-operator-test'}
export OG_PATH=${OG_PATH:-'./bundle/quay-operator.operatorgroup.yaml'}
export SUBSCRIPTION_PATH=${SUBSCRIPTION_PATH:-'./bundle/quay-operator.subscription.yaml'}
export QUAY_SAMPLE_PATH=${QUAY_SAMPLE_PATH:-'./config/samples/managed.quayregistry.yaml'}
export NAMESPACE=${NAMESPACE:-'quay-operator-e2e-nightly'}

info 'tearing down created resources...'

info 'uninstall Quay'
oc delete -n "${NAMESPACE}" -f "${QUAY_SAMPLE_PATH}"

info 'uninstalling operator'
oc delete -n "${NAMESPACE}" operatorgroup "$(yq e '.metadata.name' "${OG_PATH}")"
oc delete -n "${NAMESPACE}" subscription "$(yq e '.metadata.name' "${SUBSCRIPTION_PATH}")"
oc delete -n "${NAMESPACE}" csv "${OPERATOR_PKG_NAME}"

info 'deleting catalog source'
oc delete catsrc "${OPERATOR_PKG_NAME}" -n openshift-marketplace
