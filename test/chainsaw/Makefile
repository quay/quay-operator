## Chainsaw (e2e test runner)
CHAINSAW ?= $(LOCALBIN)/chainsaw
CHAINSAW_VERSION ?= v0.2.14
CHAINSAW_PARALLEL ?= 2
CHAINSAW_CONFIG := test/chainsaw/.chainsaw.yaml
CHAINSAW_TEST_DIR := test/chainsaw/
CHAINSAW_VALUES_OPENSHIFT := test/chainsaw/values-openshift.yaml
CHAINSAW_VALUES_KIND := test/chainsaw/values-kind.yaml
CHAINSAW_STS_TEST_DIR := test/chainsaw/sts_cco/
CHAINSAW_REPORT_FORMAT ?=
CHAINSAW_REPORT_PATH ?=
CHAINSAW_REPORT_NAME ?= chainsaw-report
CHAINSAW_REPORT_FLAGS := $(if $(CHAINSAW_REPORT_FORMAT),--report-format $(CHAINSAW_REPORT_FORMAT) --report-name $(CHAINSAW_REPORT_NAME) --report-path $(or $(CHAINSAW_REPORT_PATH),.))

.PHONY: chainsaw test-e2e test-e2e-destructive test-e2e-hpa test-e2e-kind test-e2e-sts

chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	@mkdir -p $(LOCALBIN)
	@test -s $(LOCALBIN)/chainsaw && $(LOCALBIN)/chainsaw version 2>/dev/null | grep -q $(CHAINSAW_VERSION) || \
	{ curl -sL "https://github.com/kyverno/chainsaw/releases/download/$(CHAINSAW_VERSION)/chainsaw_linux_amd64.tar.gz" | tar -xz -C $(LOCALBIN) chainsaw; }

test-e2e: chainsaw ## Run Chainsaw e2e tests (excludes destructive tests)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_OPENSHIFT) --parallel $(CHAINSAW_PARALLEL) --exclude-test-regex "/ca.rotation|/postgres.tls.lifecycle|/sts-cco" $(CHAINSAW_REPORT_FLAGS) $(CHAINSAW_EXTRA_ARGS)

test-e2e-destructive: chainsaw ## Run destructive Chainsaw e2e tests (ca-rotation, postgres-tls-lifecycle, OpenShift only)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_OPENSHIFT) --parallel $(CHAINSAW_PARALLEL) --include-test-regex "/ca.rotation|/postgres.tls.lifecycle" $(CHAINSAW_REPORT_FLAGS) $(CHAINSAW_EXTRA_ARGS)

test-e2e-kind: chainsaw ## Run Chainsaw e2e tests (KinD)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_KIND) --parallel $(CHAINSAW_PARALLEL) --exclude-test-regex "/ca.rotation|/hpa|/tls.security.profile|/postgres.tls.lifecycle|/postgres.tls.service.ca|/sts-cco" $(CHAINSAW_REPORT_FLAGS) $(CHAINSAW_EXTRA_ARGS)

test-e2e-sts: chainsaw ## Run STS/CCO tests against an AWS OpenShift cluster
	@test -n "$(STS_TEST_NAMESPACE)" || (echo "ERROR: STS_TEST_NAMESPACE is required" && exit 1)
	@test -n "$(STS_S3_BUCKET)" || (echo "ERROR: STS_S3_BUCKET is required" && exit 1)
	@test -n "$(STS_S3_REGION)" || (echo "ERROR: STS_S3_REGION is required" && exit 1)
	@test -n "$(STS_ROLE_ARN)" || (echo "ERROR: STS_ROLE_ARN is required" && exit 1)
	@kubectl get namespace "$(STS_TEST_NAMESPACE)" >/dev/null 2>&1 || kubectl create namespace "$(STS_TEST_NAMESPACE)"
	@NAMESPACE="$(STS_TEST_NAMESPACE)" STS_S3_BUCKET="$(STS_S3_BUCKET)" STS_S3_REGION="$(STS_S3_REGION)" STS_ROLE_ARN="$(STS_ROLE_ARN)" \
		$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_STS_TEST_DIR) --values $(CHAINSAW_VALUES_OPENSHIFT) --namespace "$(STS_TEST_NAMESPACE)" --parallel 1 $(CHAINSAW_REPORT_FLAGS) $(CHAINSAW_EXTRA_ARGS)
