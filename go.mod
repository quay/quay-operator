module github.com/quay/quay-operator

go 1.16

require (
	github.com/coreos/prometheus-operator v0.28.0
	github.com/go-logr/logr v0.3.0
	github.com/go-redis/redis/v8 v8.11.3 // indirect
	github.com/kube-object-storage/lib-bucket-provisioner v0.0.0-20210311161930-4bea5edaff58
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/quay/clair/v4 v4.2.2
	github.com/quay/config-tool v0.1.5-0.20210831142017-0d37fb03055b
	github.com/stretchr/testify v1.6.1
	github.com/tidwall/sjson v1.1.5
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/kustomize/api v0.5.0
	sigs.k8s.io/yaml v1.2.0
)
