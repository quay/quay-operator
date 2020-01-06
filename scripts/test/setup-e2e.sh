export QUAY_NAMESPACE="quay-enterprise"
#echo "Download oc client"
#sudo wget -qO- https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz | sudo tar -xvz -C .
#sudo mv openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit/* /bin
#echo "Bring up okd cluster"
#oc cluster up --skip-registry-check=true
echo "Login"
oc login -u system:admin
echo "Creating new project $QUAY_NAMESPACE"
oc new-project $QUAY_NAMESPACE
oc create serviceaccount quay
oc adm policy add-scc-to-user anyuid -z quay
oc adm policy add-cluster-role-to-user cluster-admin admin
oc login -u admin -p admin
oc apply -f ./deploy/crds/redhatcop.redhat.io_quayecosystems_crd-3.x.yaml