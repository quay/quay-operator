#!/bin/bash

# first, update references to versions in the CSV.
# this process is exactly the same for upstream so use the same script.
QUAY_RELEASE="${RELEASE}"
CLAIR_RELEASE="${RELEASE}"
export QUAY_RELEASE
export CLAIR_RELEASE
./hack/prepare-upstream.sh

source hack/downstream.env

awk -i inplace -v r="${DESCRIPTION}" '{gsub(/^  description: Opinionated.*/,r)}1' bundle/manifests/quay-operator.clusterserviceversion.yaml

sed -i 's|^    containerImage: quay.io/projectquay/quay-operator|    containerImage: registry-proxy.engineering.redhat.com/rh-osbs/quay-quay-operator-rhel8|' bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i 's|^    description: Opinionated deployment of Quay on Kubernetes.|    description: Opinionated deployment of Red Hat Quay on Kubernetes.|' bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i 's|^  displayName: Quay|  displayName: Red Hat Quay|' bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|quay-operator-tng|quay-operator.v${RELEASE}|" bundle/manifests/quay-operator.clusterserviceversion.yaml

sed -i "s|quay.io/projectquay/quay-operator:|registry-proxy.engineering.redhat.com/rh-osbs/quay-quay-operator-rhel8:|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|quay.io/projectquay/quay:|registry-proxy.engineering.redhat.com/rh-osbs/quay-quay-rhel8:v|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|quay.io/projectquay/clair:|registry-proxy.engineering.redhat.com/rh-osbs/quay-clair-rhel8:v|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|quay.io/projectquay/quay-builder:master|registry-proxy.engineering.redhat.com/rh-osbs/quay-quay-builder-rhel8:v${RELEASE}|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|quay.io/projectquay/quay-builder-qemu:main|registry-proxy.engineering.redhat.com/rh-osbs/quay-quay-builder-qemu-rhcos-rhel8:v${RELEASE}|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|centos/postgresql-10-centos7.*|registry.redhat.io/rhel8/postgresql-10:1|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|centos/redis-32-centos7.*|registry.redhat.io/rhel8/redis-5:1|" bundle/manifests/quay-operator.clusterserviceversion.yaml

sed -i "s|- base64data: .*|- base64data: ${LOGO}|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|value: upstream|value: redhat|" bundle/manifests/quay-operator.clusterserviceversion.yaml

sed -i "s|- email: quay-sig@googlegroups.com|- email: support@redhat.com|" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s|name: Project Quay Contributors|name: Red Hat|" bundle/manifests/quay-operator.clusterserviceversion.yaml

# remove CSV `replaces: .*` line when REPLACES is not set, or set to empty
if [ -z "${REPLACES}" ]; then
	sed -i "/replaces: .*/d" bundle/manifests/quay-operator.clusterserviceversion.yaml
else
	sed -i "s|replaces: .*|replaces: quay-operator.v${REPLACES}|" bundle/manifests/quay-operator.clusterserviceversion.yaml
fi

sed -i "s|quay-operator-tng|quay-operator|" bundle/metadata/annotations.yaml
