package e2e

import (
	goctx "context"
	"testing"

	test "github.com/redhat-cop/quay-operator/test"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func DisabledTestClairConfiguration(t *testing.T) {
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

	// TODO - REMOVE personal quay repo reference
	quayEcosystem.Spec.Quay.Image = "quay.io/cnuland/quay:deval"
	// Get the host IP and then specify the external route for quay
	//ip := strings.Trim(strings.Split(f.KubeConfig.Host, ":")[1], "//")
	//quayEcosystem.Spec.Quay.ConfigRouteHost = "quay-operator-quay-config-quay-enterprise." + ip + ".nip.io"

	// Add clair enabled, image pull secret, and the name
	// TODO - Make Clair work in a CI environment. Currently broken because of subpath mounting broken in minishift https://github.com/openshift/origin/issues/21404
	quayEcosystem.Spec.Clair = &redhatcopv1alpha1.Clair{
		Enabled: true,
		Image:   "quay.io/cnuland/clair:latest",
	}
	quayEcosystem.ObjectMeta.Name = name
	quayEcosystem.ObjectMeta.Namespace = namespace
	err = f.Client.Create(goctx.TODO(), quayEcosystem, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	assert.NoError(t, err)

	// Check if the CR has been created
	cr := &redhatcopv1alpha1.QuayEcosystem{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cr)
	assert.NoError(t, err)

	//Wait for the redis pod deployment
	success := WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-redis", constants.RedisImage, retryInterval, timeout)
	assert.NoError(t, success)
	// Verify the crd has been given default values
	cr = &redhatcopv1alpha1.QuayEcosystem{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cr)
	assert.NoError(t, err)
	// Wait for the postgresql deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-postgresql", constants.PostgresqlImage, retryInterval, timeout)
	assert.NoError(t, success)
	// Wait for the quay deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-config", "quay.io/cnuland/quay:deval", retryInterval, timeout)
	assert.NoError(t, success)
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay", "quay.io/cnuland/quay:deval", retryInterval, timeout)
	assert.NoError(t, success)
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-clair", "quay.io/cnuland/clair:latest", retryInterval, timeout)
	assert.NoError(t, success)
}
