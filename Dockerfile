FROM registry.access.redhat.com/ubi8/go-toolset:1.19 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor

COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 go build -mod vendor -o manager main.g

FROM scratch
WORKDIR /workspace
COPY --from=builder /workspace/manager manager
COPY kustomize/ kustomize/

ENTRYPOINT ["/workspace/manager"]
