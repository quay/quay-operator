#REMOVE BEFORE MERGING
QUAY_PASSWORD='O81WSHRSJR14UAZBK54GQHJS0P1V4CLWAJV1X2C4SD7KO59CQ9N3RE12612XU1HR' 
QUAY_USERNAME='redhat+quay'
RH_PASSWORD='maythe(ma)bwithu22!'
RH_USERNAME='RHN-GPS-cnuland'

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

    echo "Logging into redhat registry"
    docker login registry.redhat.io -u $RH_USERNAME -p $RH_PASSWORD
    echo "Bring up openshift cluster"
    ./oc cluster up
    ./oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    ./oc new-project $QUAY_NAMESPACE
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    ./oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
fi
