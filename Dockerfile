FROM golang:1.17 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor

COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

RUN ARCH=$(uname -m) ; if [[ $ARCH == "x86_64" ]] ; then ARCH = "amd64" ; fi
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH GO111MODULE=on go build -mod vendor -o manager main.go

FROM scratch
WORKDIR /workspace
COPY --from=builder /workspace/manager manager
COPY kustomize/ kustomize/

ENTRYPOINT ["/workspace/manager"]
