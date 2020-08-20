module github.com/quay/quay-operator

go 1.1

require (
	github.com/go-logr/logr v0.1.0
	github.com/kube-object-storage/lib-bucket-provisioner v0.0.0-20200610144127-e2eec875d6d1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/openshift/api v3.9.0+incompatible
	github.com/quay/clair/v4 v4.0.0-alpha.7.0.20200717155243-2238b246a10b
	github.com/quay/config-tool v0.1.2-0.20200805231543-34b46f1ae510
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/api v0.5.0
	sigs.k8s.io/yaml v1.2.0
)
