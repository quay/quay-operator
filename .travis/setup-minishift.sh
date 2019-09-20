#!/bin/bash
if [[ -z "${QUAY_PASSWORD}" ]]; then
    echo "QUAY_PASSWORD environment variable not set"
elif [[ -z "${QUAY_USERNAME}" ]]; then
    echo "QUAY_USERNAME environment variable not set"
else
    echo "Download oc client"
    sudo wget -qO- https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz | sudo tar -xvz -C .
    sudo mv openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit/* /bin
    echo "Logging into quay.io"
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    cp ~/.docker/config.json ./
    echo "Bring up okd cluster"
    oc cluster up --skip-registry-check=true
    echo "Login"
    oc login -u system:admin
    echo "Creating new project $QUAY_NAMESPACE"
    oc new-project $QUAY_NAMESPACE
    oc create serviceaccount quay
    oc adm policy add-scc-to-user anyuid -z quay
    oc adm policy add-cluster-role-to-user cluster-admin admin
    oc login -u admin -p admin
    oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=config.json" --type='kubernetes.io/dockerconfigjson'
    oc apply -f ./deploy/crds/redhatcop_v1alpha1_quayecosystem_crd.yaml
fi
