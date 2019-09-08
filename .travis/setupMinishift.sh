if [[ -z "${QUAY_PASSWORD}" ]]; then
    echo "QUAY_PASSWORD environment variable not set"
elif [[ -z "${QUAY_USERNAME}" ]]; then
    echo "QUAY_USERNAME environment variable not set"
else
    echo "Logging into quay as $QUAY_USERNAME"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
fi
