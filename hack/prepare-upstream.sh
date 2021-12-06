sed -i "s/createdAt:.*/createdAt: `date -u +'%Y-%m-%d %k:%m UTC'`/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/olm\.skipRange:.*/olm\.skipRange: \">=3.3.x <${RELEASE}\"/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/quay-version:.*/quay-version: ${RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/containerImage:.*/containerImage: quay.io\/projectquay\/quay-operator:v${RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/^  name: quay-operator.*/  name: quay-operator.v${RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/image: quay.io\/projectquay\/quay-operator.*/image: quay.io\/projectquay\/quay-operator:v${RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/value: quay.io\/projectquay\/quay:.*/value: quay.io\/projectquay\/quay:${QUAY_RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/value: quay.io\/projectquay\/clair:.*/value: quay.io\/projectquay\/clair:${CLAIR_RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/^  version: .*/  version: ${RELEASE}/" bundle/manifests/quay-operator.clusterserviceversion.yaml
sed -i "s/operators.operatorframework.io.bundle.channel.default.v1.*/operators.operatorframework.io.bundle.channel.default.v1: ${CHANNEL}/" bundle/metadata/annotations.yaml
sed -i "s/operators.operatorframework.io.bundle.channels.v1.*/operators.operatorframework.io.bundle.channels.v1: ${CHANNEL}/" bundle/metadata/annotations.yaml
