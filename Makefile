
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	QUAY_VERSION=dev go run main.go --cert-dir ${PWD}/config/webhook

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# This target called from the prepare-release github action.
# RELEASE       - quay-operator tag (eg. v3.6.0-alpha.4)
# QUAY_RELEASE  - quay version
# CLAIR_RELEASE - clair version
prepare-release:
	sed -i "s/createdAt:.*/createdAt: `date --utc +'%Y-%m-%d %k:%m UTC'`/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/olm\.skipRange:.*/olm\.skipRange: \">=3.5.x <$(RELEASE)\"/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/quay-version:.*/quay-version: v$(RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/containerImage:.*/containerImage: quay.io\/projectquay\/quay-operator:v$(RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/^  name: quay-operator.*/  name: quay-operator.v$(RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/image: quay.io\/projectquay\/quay-operator.*/image: quay.io\/projectquay\/quay-operator:v$(RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/value: quay.io\/projectquay\/quay:.*/value: quay.io\/projectquay\/quay:$(QUAY_RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/value: quay.io\/projectquay\/clair:.*/value: quay.io\/projectquay\/clair:$(CLAIR_RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
	sed -i "s/^  version: .*/  version: $(RELEASE)/" bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml
ifneq (,$(findstring alpha,$(RELEASE)))
	sed -i "s/operators.operatorframework.io.bundle.channel.default.v1.*/operators.operatorframework.io.bundle.channel.default.v1: preview-3.6/" bundle/upstream/metadata/annotations.yaml
	sed -i "s/operators.operatorframework.io.bundle.channels.v1.*/operators.operatorframework.io.bundle.channels.v1: preview-3.6/" bundle/upstream/metadata/annotations.yaml
else
	sed -i "s/operators.operatorframework.io.bundle.channel.default.v1.*/operators.operatorframework.io.bundle.channel.default.v1: stable-3.6/" bundle/upstream/metadata/annotations.yaml
	sed -i "s/operators.operatorframework.io.bundle.channels.v1.*/operators.operatorframework.io.bundle.channels.v1: stable-3.6/" bundle/upstream/metadata/annotations.yaml
endif
