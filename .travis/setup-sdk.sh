#!/bin/bash
#https://github.com/jaegertracing/jaeger-operator/blob/master/.ci/install-sdk.sh

export DEST="${GOPATH}/bin/operator-sdk"
export SDK_VERSION=v0.10.0

mkdir -p ${GOPATH}/bin
echo "Downloading the operator-sdk ${SDK_VERSION} into ${DEST}"
url https://github.com/operator-framework/operator-sdk/releases/download/${SDK_VERSION}/operator-sdk-${SDK_VERSION}-x86_64-linux-gnu -sLo ${DEST}
chmod +x ${DEST}