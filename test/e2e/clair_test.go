package e2e

import (
	goctx "context"
	"testing"

	test "github.com/redhat-cop/quay-operator/test"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/types"
)

func TestClairConfiguration(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	test.AddToFrameworkSchemeForTests(t, ctx)
	defaultClairDeploy(t, framework.Global, ctx)
}

func defaultClairDeploy(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	assert.NoError(t, err)

	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	changed := validation.SetDefaults(f.Client.Client, &quayConfiguration)
	assert.Equal(t, changed, true)

	// Add clair enabled, image pull secret, and the name
	quayEcosystem.Spec.Quay.ImagePullSecretName = "redhat-pull-secret"
	quayEcosystem.Spec.Quay.ConfigRouteHost = "quay-operator-quay-config-quay-enterprise.192.168.99.101.nip.io"
	quayEcosystem.Spec.Clair = &redhatcopv1alpha1.Clair{
		Enabled:             true,
		ImagePullSecretName: "redhat-pull-secret",
		Image:               constants.ClairImage,
	}
	quayEcosystem.ObjectMeta.Name = name
	quayEcosystem.ObjectMeta.Namespace = namespace

	err = f.Client.Create(goctx.TODO(), quayEcosystem, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	assert.NoError(t, err)

	// Check if the CRD has been created
	crd := &redhatcopv1alpha1.QuayEcosystem{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, crd)

	assert.NoError(t, err)

	//Wait for the redis pod deployment
	success := WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-redis", "registry.access.redhat.com/rhscl/redis-32-rhel7:latest", retryInterval, timeout)
	assert.NoError(t, success)

	// Verify the crd has been given default values
	crd = &redhatcopv1alpha1.QuayEcosystem{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, crd)
	assert.NoError(t, err)
	assert.Equal(t, crd.Spec.Quay.Image, constants.QuayImage)

	// Wait for the quay deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-config", "quay.io/redhat/quay:v3.0.4", retryInterval, timeout)
	assert.NoError(t, success)

	// Wait for the postgresql deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-postgresql", "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1", retryInterval, timeout)
	assert.NoError(t, success)

	// Wait for the clair postgres deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-clair-postgresql", "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1", retryInterval, timeout)
	assert.NoError(t, success)
}
