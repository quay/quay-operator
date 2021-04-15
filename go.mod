module github.com/quay/quay-operator

go 1.12

require (
	github.com/coreos/prometheus-operator v0.28.0
	github.com/go-logr/logr v0.3.0
	github.com/kube-object-storage/lib-bucket-provisioner v0.0.0-20200610144127-e2eec875d6d1
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/openshift/api v3.9.0+incompatible
	github.com/quay/clair/v4 v4.0.0-rc.20.0.20201112172303-bb3cd669f663
	github.com/quay/claircore v1.0.5 // indirect
	github.com/quay/config-tool v0.1.2-0.20210118162351-e19851d40f9e
	github.com/stretchr/testify v1.6.1
	github.com/tidwall/sjson v1.1.5
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/kustomize/api v0.5.0
	sigs.k8s.io/yaml v1.2.0
)
