FROM golang:1.16 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor

COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod vendor -o manager main.go

FROM scratch
WORKDIR /workspace
COPY --from=builder /workspace/manager manager
COPY kustomize/ kustomize/

ENTRYPOINT ["/workspace/manager"]
