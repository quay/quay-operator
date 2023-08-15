#!/usr/bin/env bash
# deploys a ppc64le cluster on IBM CLoud Power Virtual Server
# using the a stable openshift-install binary for ppc64le.
#
# REQUIREMENTS:
#  * env variable `IBMCLOUD_API_KEY`
#  * `oc` binary
set -e

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

# prints pre-formatted error output.
function error {
	>&2 echo "ERROR $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

RESOURCE_GROUP=Default
CLUSTER_ID=quay-e2e
OCP_INSTALL_DIR=quaye2e
CCO_DIR=ccodir
PULL_SEC_LOC=~/.pull-secret

info 'oc client version:'
oc version

# Install openshift-install amd64 version and extract ccoctl
curl -LO https://mirror.openshift.com/pub/openshift-v4/amd64/clients/ocp/stable-4.13/openshift-install-linux.tar.gz
tar zxvf openshift-install-linux.tar.gz
info 'openshift-install version:'
./openshift-install version
info 'extracting ccoctl with amd64 binary...'
RELEASE_IMAGE=$(./openshift-install version | awk '/release image/ {print $3}')
CCO_IMAGE=$(oc adm release info --image-for='cloud-credential-operator' "$RELEASE_IMAGE" -a $PULL_SEC_LOC)
oc image extract "$CCO_IMAGE" --file="/usr/bin/ccoctl" -a $PULL_SEC_LOC
chmod 775 ccoctl



if [[ -d $OCP_INSTALL_DIR &&  -f $OCP_INSTALL_DIR/install-config.yaml ]]; then
    info 'found install config'
else
    error 'missing install config or dir'
    exit 1
fi

# delete the amd64 version of openshift-install and install ppc64le version
rm -f openshift-install-linux.tar.gz openshift-install
curl -LO https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/stable-4.13/openshift-install-linux-amd64.tar.gz
tar zxvf openshift-install-linux-amd64.tar.gz
info 'openshift-install version:'
./openshift-install version
info 'creating manifests...'
./openshift-install create manifests --dir $OCP_INSTALL_DIR --log-level info


info 'extracting cco request objects...'
oc adm release extract --cloud=powervs --credentials-requests "$RELEASE_IMAGE" --to=$CCO_DIR
./ccoctl ibmcloud create-service-id --credentials-requests-dir $CCO_DIR --name $CLUSTER_ID --output-dir $OCP_INSTALL_DIR --resource-group-name $RESOURCE_GROUP &> /dev/null


info 'creating cluster...'
./openshift-install create cluster --dir $OCP_INSTALL_DIR --log-level=info
