#REMOVE BEFORE MERGING
export QUAY_PASSWORD="O81WSHRSJR14UAZBK54GQHJS0P1V4CLWAJV1X2C4SD7KO59CQ9N3RE12612XU1HR"
export QUAY_USERNAME="redhat+quay"
export KUBE_URL="https://storage.googleapis.com/kubernetes-release/release/v1.10.13/bin/linux/amd64/kubectl"
if [[ -z "${QUAY_PASSWORD}" ]]; then
    echo "QUAY_PASSWORD environment variable not set"
elif [[ -z "${QUAY_USERNAME}" ]]; then
    echo "QUAY_USERNAME environment variable not set"
elif [[ -z "${RH_PASSWORD}" ]]; then
    echo "RH_PASSWORD environment variable not set"
elif [[ -z "${RH_USERNAME}" ]]; then
    echo "RH_USERNAME environment variable not set"
else
    sudo wget -qO- https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz | sudo tar /openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit -xvz -C /bin
    sudo ls /bin
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./

    echo "Bring up openshift cluster"
    IP_ADDR=$(ip addr show $DEV | awk '/inet /{ gsub("/.*", ""); print $2}')
    oc cluster up --public-hostname=${IP_ADDR} --routing-suffix=${IP_ADDR}.nip.io --base-dir=/home/travis/ocp --skip-registry-check=true
    oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    oc new-project $QUAY_NAMESPACE
    oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
    oc apply -f ./deploy/crd/redhatcop_v1alpha1_quayecosystem_crd.yaml
fi
