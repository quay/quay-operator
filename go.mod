module github.com/quay/quay-operator

go 1.1

require (
	github.com/go-logr/logr v0.1.0
	github.com/kube-object-storage/lib-bucket-provisioner v0.0.0-20200610144127-e2eec875d6d1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/openshift/api v3.9.0+incompatible
	github.com/quay/clair/v4 v4.0.0-rc.18.0.20201022192047-157628dfe1c7
	github.com/quay/claircore v1.0.5 // indirect
	github.com/quay/config-tool v0.1.2-0.20201013214416-e1ea29372174
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200506231410-2ff61e1afc86
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/api v0.5.0
	sigs.k8s.io/yaml v1.2.0
)
