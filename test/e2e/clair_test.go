package e2e

import (
	goctx "context"
	"strings"
	"testing"
	"time"

	test "github.com/redhat-cop/quay-operator/test"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/validation"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	retryInterval        = time.Second * 120
	timeout              = time.Second * 460
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
	name                 = "quay-operator"
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

	// Get the host IP and then specify the external route for quay
	quayEcosystem.Spec.Quay.ImagePullSecretName = "redhat-pull-secret"
	ip := strings.Trim(strings.Split(f.KubeConfig.Host, ":")[1], "//")
	quayEcosystem.Spec.Quay.ConfigRouteHost = "quay-operator-quay-config-quay-enterprise." + ip + ".nip.io"

	// Add clair enabled, image pull secret, and the name
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
	success := WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-redis", constants.RedisImage, retryInterval, timeout)
	assert.NoError(t, success)
	// Verify the crd has been given default values
	crd = &redhatcopv1alpha1.QuayEcosystem{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, crd)
	assert.NoError(t, err)
	assert.Equal(t, crd.Spec.Quay.Image, constants.QuayImage)
	// Wait for the postgresql deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-postgresql", constants.PostgresqlImage, retryInterval, timeout)
	assert.NoError(t, success)
	// Wait for the quay deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay-config", constants.QuayImage, retryInterval, timeout)
	assert.NoError(t, success)
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-quay", constants.QuayImage, retryInterval, timeout)
	assert.NoError(t, success)
	// Wait for the clair postgres deployment
	success = WaitForPodWithImage(t, f, ctx, namespace, "quay-operator-clair-postgresql", constants.PostgresqlImage, retryInterval, timeout)
	assert.NoError(t, success)
	// NOTE: Because of limitations with mounting subPath in minishift we must check for the deployment of clair instead of the pod
	//Check for the clair deployment
	deployment := &appsv1.Deployment{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "quay-operator-clair", Namespace: namespace}, deployment)
	assert.NoError(t, err)

}
