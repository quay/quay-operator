FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=quay-operator
LABEL operators.operatorframework.io.bundle.channels.v1=quay-v3.5
LABEL operators.operatorframework.io.bundle.channel.default.v1=quay-v3.5

COPY manifests /manifests/
COPY metadata/annotations.yaml /metadata/annotations.yaml
