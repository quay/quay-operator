#!/usr/bin/env bash
# destroy a ppc64le cluster on IBM CLoud Power Virtual Server
# using the a stable openshift-install binary for ppc64le.
#
# REQUIREMENTS:
#  * env variable `IBMCLOUD_API_KEY`

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

CLUSTER_ID=quay-e2e
OCP_INSTALL_DIR=quaye2e
CCO_DIR=ccodir

if [[ -f ./ccoctl ]]; then
	info 'deleting cco request objects...'
	./ccoctl ibmcloud delete-service-id --credentials-requests-dir $CCO_DIR --name $CLUSTER_ID
fi

if [[ -f ./openshift-install ]]; then
	info 'destroying the cluster...'
	./openshift-install destroy cluster --dir $OCP_INSTALL_DIR --log-level=info
fi
