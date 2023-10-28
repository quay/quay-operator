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


NO_OF_RETRY=${NO_OF_RETRY:-"3"}
function retry {
  cmd=$1
  for i in $(seq 1 "$NO_OF_RETRY"); do
    echo "Attempt: $i/$NO_OF_RETRY"
    ret_code=0
    $cmd || ret_code=$?
    if [ $ret_code = 0 ]; then
      break
    elif [ "$i" == "$NO_OF_RETRY" ]; then
      echo "All retry attempts failed!"
      exit $ret_code
    else
      sleep 10
    fi
  done
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
	retry "./openshift-install destroy cluster --dir $OCP_INSTALL_DIR --log-level=info"
fi
