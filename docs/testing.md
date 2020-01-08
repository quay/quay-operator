# Testing

The Quay Operator has both unit and E2E tests that validate a variety of functionality within the operator. This document outlines the test suite of this project and outlines how a user can run tests locally.

## Running Unit Tests

These unit tests can be run within the standard go library and do not require the operator to be running. All operator functionality is handled by the fake client as described here,
https://github.com/operator-framework/operator-sdk/blob/master/doc/user/unit-testing.md#using-a-fake-client


```bash
# Run within the root project folder
go test -v ./pkg/... 

```

### Running E2E Tests

The E2E tests are handled by the Operator SDK and must be run against a live K8 based instance. The below example will run the operator within a local 3.x okd instance.

```bash
# Run within the root project folder
# Start a local instance
echo "Starting OKD"
oc cluster up --skip-registry-check=true
echo "Login"
oc login -u system:admin
echo "Creating new project quay-enterprise"
oc new-project "quay-enterprise"
oc create serviceaccount quay
oc adm policy add-scc-to-user anyuid -z quay
oc adm policy add-cluster-role-to-user cluster-admin admin
oc login -u admin -p admin
# If running a 3.x instance
oc apply -f ./deploy/crds/redhatcop.redhat.io_quayecosystems_crd-3.x.yaml
# Run an instance of the operator
operator-sdk up local --namespace=quay-enterprise
# Run the E2E tests in a seperate tab or screen instance
operator-sdk test local ./test/e2e --namespace "quay-enterprise" --up-local --no-setup --verbose
```