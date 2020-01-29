#!/bin/bash

trap popd >> /dev/null 2>&1 

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

pushd "${DIR}/../../" >> /dev/null 2>&1 


set +e

if ! command -v yq > /dev/null 2>&1; then
  echo given-command is not available
  exit 1
fi

set -e

operator-sdk generate k8s
operator-sdk generate openapi

cp -f "deploy/crds/redhatcop.redhat.io_quayecosystems_crd.yaml" "deploy/crds/redhatcop.redhat.io_quayecosystems_crd-3.x.yaml"

# Remove Invalid Property
yq d -i "deploy/crds/redhatcop.redhat.io_quayecosystems_crd-3.x.yaml" spec.validation.openAPIV3Schema.type