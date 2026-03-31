## Chainsaw (e2e test runner)
CHAINSAW ?= $(LOCALBIN)/chainsaw
CHAINSAW_VERSION ?= v0.2.14
CHAINSAW_PARALLEL ?= 2
CHAINSAW_CONFIG := test/chainsaw/.chainsaw.yaml
CHAINSAW_TEST_DIR := test/chainsaw/
CHAINSAW_VALUES_OPENSHIFT := test/chainsaw/values-openshift.yaml
CHAINSAW_VALUES_KIND := test/chainsaw/values-kind.yaml

.PHONY: chainsaw test-e2e test-e2e-destructive test-e2e-hpa test-e2e-kind

chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	@mkdir -p $(LOCALBIN)
	@test -s $(LOCALBIN)/chainsaw && $(LOCALBIN)/chainsaw version 2>/dev/null | grep -q $(CHAINSAW_VERSION) || \
	{ curl -sL "https://github.com/kyverno/chainsaw/releases/download/$(CHAINSAW_VERSION)/chainsaw_linux_amd64.tar.gz" | tar -xz -C $(LOCALBIN) chainsaw; }

test-e2e: chainsaw ## Run Chainsaw e2e tests (excludes destructive tests)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_OPENSHIFT) --parallel $(CHAINSAW_PARALLEL) --exclude-test-regex "/ca.rotation" $(CHAINSAW_EXTRA_ARGS)

test-e2e-destructive: chainsaw ## Run destructive Chainsaw e2e tests (ca-rotation, OpenShift only)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_OPENSHIFT) --parallel $(CHAINSAW_PARALLEL) --include-test-regex "/ca.rotation" $(CHAINSAW_EXTRA_ARGS)

test-e2e-kind: chainsaw ## Run Chainsaw e2e tests (KinD)
	$(CHAINSAW) test --config $(CHAINSAW_CONFIG) --test-dir $(CHAINSAW_TEST_DIR) --values $(CHAINSAW_VALUES_KIND) --parallel $(CHAINSAW_PARALLEL) --exclude-test-regex "/ca.rotation|/hpa|/unmanaged_postgres" --assert-timeout 20m $(CHAINSAW_EXTRA_ARGS)
