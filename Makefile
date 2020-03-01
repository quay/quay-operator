
# Image URL to use all building/pushing image targets
REGISTRY ?= quay.io
REPOSITORY ?= $(REGISTRY)/redhat-cop/quay-operator
IMG := $(REPOSITORY):latest

VERSION := v1.0.2

BUILD_COMMIT := $(shell ./scripts/build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./scripts/build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./scripts/build/get-build-hostname.sh)
CUSTOM_TAG ?= $(BUILD_COMMIT)

LDFLAGS := "-X github.com/redhat-cop/quay-operator/version.Version=$(VERSION) \
	-X github.com/redhat-cop/quay-operator/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/redhat-cop/quay-operator/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/redhat-cop/quay-operator/version.Hostname=$(BUILD_HOSTNAME)"

all: manager

# Run tests
native-test: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o build/_output/bin/quay-operator  -ldflags $(LDFLAGS) github.com/redhat-cop/quay-operator/cmd/manager

# Build manager binary
manager-osx: generate fmt vet
	go build -o build/_output/bin/quay-operator GOOS=darwin  -ldflags $(LDFLAGS) github.com/redhat-cop/quay-operator/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install:
	cat deploy/crds/*crd.yaml | kubectl apply -f-

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Docker Login
docker-login:
	@docker login -u $(DOCKER_USER) -p $(DOCKER_PASSWORD) $(REGISTRY)

# Custom Tag
docker-tag-custom-tag:
	@docker tag $(IMG) $(REPOSITORY):$(CUSTOM_TAG)

# Tag for Dev
docker-tag-release:
	@docker tag $(IMG) $(REPOSITORY):$(VERSION)
	@docker tag $(IMG) $(REPOSITORY):latest	

# Push for Dev
docker-push-custom-tag:  docker-tag-custom-tag
	@docker push $(REPOSITORY):$(CUSTOM_TAG)

# Push for Release
docker-push-release:  docker-tag-release
	@docker push $(REPOSITORY):$(VERSION)
	@docker push $(REPOSITORY):latest

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# CI Latest Tag Deployment
ci-latest-deploy: docker-login docker-build docker-push

# CI Custom Tag Deployment
ci-custom-tag-deploy: docker-login docker-build docker-push-custom-tag

# CI Release
ci-release-deploy: docker-login docker-build docker-push-release