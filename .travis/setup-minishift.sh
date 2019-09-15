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
    wget https://mirror.openshift.com/pub/openshift-v3/clients/${OPENSHIFT_VERSION}/linux/oc.tar.gz
    tar xvzf oc.tar.gz

    echo "Clean up"
    ./oc cluster down
    rm -rf /etc/origin;mkdir -p /etc/origin ~/.kube

    echo "Bring up openshift cluster"
    ./oc cluster up --image=registry.access.redhat.com/openshift3/ose-control-plane:v3.11
    ./oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    ./oc new-project $QUAY_NAMESPACE
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    ./oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
fi
