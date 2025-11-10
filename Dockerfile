FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

ARG TARGETOS TARGETARCH
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor

COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

ENV GOEXPERIMENT=strictfipsruntime \
    CGO_ENABLED=1 

RUN go build -tags strictfipsruntime -mod vendor -o manager main.go

FROM registry.access.redhat.com/ubi9/ubi-micro:latest AS base

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS installer
COPY --from=base / /mnt/rootfs/

RUN set -ex \
    && MICRODNF_OPTS="--installroot=/mnt/rootfs --config=/etc/dnf/dnf.conf --noplugins --setopt=reposdir=/etc/yum.repos.d --setopt=cachedir=/var/cache/dnf --setopt=varsdir=/etc/dnf/vars --setopt=tsflags=nodocs" \
    && microdnf $MICRODNF_OPTS install -y  openssl-libs \
    && microdnf $MICRODNF_OPTS clean all \
    && rm -rf /var/cache/dnf /mnt/rootfs/var/cache/dnf

FROM registry.access.redhat.com/ubi9/ubi-micro:latest AS final

COPY --from=installer /mnt/rootfs /

WORKDIR /workspace
COPY --from=builder /workspace/manager manager
COPY kustomize/ kustomize/

ENTRYPOINT ["/workspace/manager"]
