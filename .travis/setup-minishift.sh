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
    sudo wget -qO- https://mirror.openshift.com/pub/openshift-v3/clients/${OPENSHIFT_VERSION}/linux/oc.tar.gz | sudo tar -xvz -C /bin
    tar xvzf oc.tar.gz
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    #docker pull quay.io/openshift/origin-node:v3.11
    #sudo docker cp $(docker create docker.io/openshift/origin:v3.11):/bin/oc /usr/local/bin/oc

    echo "Bring up openshift cluster"
    IP_ADDR=$(ip addr show $DEV | awk '/inet /{ gsub("/.*", ""); print $2}')
    oc cluster up --public-hostname=${IP_ADDR} --routing-suffix=${IP_ADDR}.nip.io --base-dir=/home/travis/ocp
    oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    oc new-project $QUAY_NAMESPACE
    oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
fi
