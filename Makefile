
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
QUAY_OPERATOR_CONTAINER_TAG ?= "quay.io/projectquay/quay-operator:v3.5.0"
QUAY_OPERATOR_CONTAINER_TAG_SED ?= "quay.io\/projectquay\/quay-operator:v3.5.0"
QUAY_OPERATOR_BUNDLE_CONTAINER_TAG ?= "quay.io/projectquay/quay-operator-bundle:v3.5.0"
QUAY_OPERATOR_INDEX_CONTAINER_TAG ?= "quay.io/projectquay/quay-operator-index:v3.5.0"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

manager: generate fmt vet
	go build -o bin/manager main.go

run: generate fmt vet manifests
	QUAY_VERSION=dev go run main.go

kubebuilder:
	os=$(go env GOOS)
	arch=$(go env GOARCH)
	curl -L https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/
	mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder
	export PATH=$PATH:/usr/local/kubebuilder/bin  

# The `make index` target will:
#   1) Build the Operator controller container image and push to a registry.
#   2) Build the Operator bundle image and push to a registry.
#   3) Build the Operator index image and push to a registry.
index: 
	docker build -t $(QUAY_OPERATOR_CONTAINER_TAG) .
	docker push $(QUAY_OPERATOR_CONTAINER_TAG)
	# FIXME(alecmerdler): This is just editing in-memory and the changes won't be present in the next `docker build` step...
	cat bundle/upstream/manifests/quay-operator.clusterserviceversion.yaml | sed "s/quay.io\/projectquay\/quay-operator:v3.5.0/${QUAY_OPERATOR_CONTAINER_TAG_SED}/"
	docker build -t $(QUAY_OPERATOR_BUNDLE_CONTAINER_TAG) .
	docker push $(QUAY_OPERATOR_BUNDLE_CONTAINER_TAG)
	opm index add --bundles $(QUAY_OPERATOR_BUNDLE_CONTAINER_TAG) --tag $(QUAY_OPERATOR_INDEX_CONTAINER_TAG) --container-tool=docker
	docker push $(QUAY_OPERATOR_INDEX_CONTAINER_TAG)

catalog: index
  cat bundle/quay-operator.catalogsource.yaml | sed "s/quay.io\/projectquay\/quay-operator:v3.5.0/$(QUAY_OPERATOR_INDEX_CONTAINER_TAG)/"
	kubectl create -n $(QUAY_OPERATOR_CATALOG_NAMESPACE) -f bundle/quay-operator.catalogsource.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt:
	go fmt ./...

vet:
	go vet ./...

generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
