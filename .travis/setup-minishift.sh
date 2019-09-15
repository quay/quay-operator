#REMOVE BEFORE MERGING
export QUAY_PASSWORD="O81WSHRSJR14UAZBK54GQHJS0P1V4CLWAJV1X2C4SD7KO59CQ9N3RE12612XU1HR"
export QUAY_USERNAME="redhat+quay"

if [[ -z "${QUAY_PASSWORD}" ]]; then
    echo "QUAY_PASSWORD environment variable not set"
elif [[ -z "${QUAY_USERNAME}" ]]; then
    echo "QUAY_USERNAME environment variable not set"
elif [[ -z "${RH_PASSWORD}" ]]; then
    echo "RH_PASSWORD environment variable not set"
elif [[ -z "${RH_USERNAME}" ]]; then
    echo "RH_USERNAME environment variable not set"
else
    #wget https://mirror.openshift.com/pub/openshift-v3/clients/${OPENSHIFT_VERSION}/linux/oc.tar.gz
    #tar xvzf oc.tar.gz
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    docker pull quay.io/openshift/origin-node:v3.11
    sudo docker cp $(docker create docker.io/openshift/origin:v3.11):/bin/oc /usr/local/bin/oc

    echo "Bring up openshift cluster"
    mkdir -p "$HOME/.occluster"
    rm -r ~/.kube
    oc cluster up  --base-dir="$HOME/.occluster" --image=registry.access.redhat.com/openshift3/ose-control-plane:v3.11
    oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    oc new-project $QUAY_NAMESPACE
    oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
fi
