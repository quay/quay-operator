package test

import (
	"testing"

	"github.com/redhat-cop/quay-operator/pkg/apis"

	ossecurityv1 "github.com/openshift/api/security/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var name = "quay-operator"
var operator = "quay-operator"
var log = logf.Log.WithName("cmd")

// Add required objects for test
var ServiceAccount = &corev1.ServiceAccount{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "clair",
		Namespace: "quay",
	},
}

var Secret = &corev1.Secret{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: corev1.SchemeGroupVersion.String(),
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "redhat-pull-secret",
		Namespace: "quay",
	},
	StringData: map[string]string{".dockerconfigjson": "UmVhbGx5IHJlYWxseSByZWVlZWVlZWVlZWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWxsbGxsbGxsbGxsbGxsbGxsbGxsbGxsbGxsbGxsbGx5eXl5eXl5eXl5eXl5eXl5eXl5eSBsbGxsbGxsbGxsbGxsbG9vb29vb29vb29vb29vb29vb29vb29vb29vb25ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubmdnZ2dnZ2dnZ2dnZ2dnZ2dnZ2cgYXV0aCBrZXlzCg"},
}

var SCCAnyUID = &ossecurityv1.SecurityContextConstraints{
	TypeMeta: metav1.TypeMeta{
		Kind:       "SecurityContextConstraints",
		APIVersion: "security.openshift.io/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "anyuid",
		Namespace: "quay",
	},
	SELinuxContext: ossecurityv1.SELinuxContextStrategyOptions{
		Type: "MustRunAs",
	},
	RunAsUser: ossecurityv1.RunAsUserStrategyOptions{
		Type: "RunAsAny",
	},
	SupplementalGroups: ossecurityv1.SupplementalGroupsStrategyOptions{
		Type: "RunAsAny",
	},
	FSGroup: ossecurityv1.FSGroupStrategyOptions{
		Type: "RunAsAny",
	},
	Users: []string{"system:serviceaccount:quaytest:clair"},
}

var PostgresPod = &corev1.Pod{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "clair-operator-postgres-g235gs",
		Namespace: "quay",
		Labels:    map[string]string{"quay-enterprise-component": "quay-database"},
	},
}

func SetupLogging() {
	// Setup logging
	// Add the zap logger flag set to the CLI. The flag set must be added before calling pflag.Parse().
	//pflag.CommandLine.AddFlagSet(zap.FlagSet())
	//pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	//pflag.Parse()
	//logf.SetLogger(zap.Logger())

}

func AddToFrameworkSchemeForTests(t *testing.T, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	assert.NoError(t, err)
	// Stub out object placeholders for test

	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{
		TypeMeta: metav1.TypeMeta{
			Kind:       "QuayEcosystem",
			APIVersion: "redhatcop.redhat.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   redhatcopv1alpha1.QuayEcosystemSpec{},
		Status: redhatcopv1alpha1.QuayEcosystemStatus{},
	}

	assert.NoError(t, framework.AddToFrameworkScheme(apis.AddToScheme, quayEcosystem))
}
