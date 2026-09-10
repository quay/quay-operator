package controllers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

func TestReadOnlyServiceKeySecretGeneration(t *testing.T) {
	quay := &v1.QuayRegistry{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "ns"}}

	secret, kid, err := generateReadOnlySecret(quay)
	require.NoError(t, err)
	require.NotEmpty(t, kid)
	assert.Equal(t, "registry-readonly-service-key", secret.Name)
	assert.True(t, *secret.Immutable)
	assert.Equal(t, kid, string(secret.Data[kustomize.ReadOnlyKIDKey]))
	assert.NotEmpty(t, secret.Data[kustomize.ReadOnlyPEMKey])

	gotKID, err := validateReadOnlySecret(secret)
	require.NoError(t, err)
	assert.Equal(t, kid, gotKID)
}

func TestOwnedByQuayRequiresMatchingUID(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry",
			Namespace: "ns",
			UID:       types.UID("quay-uid"),
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      readOnlySecretName(quay),
			Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "QuayRegistry",
				Name: "registry",
				UID:  types.UID("quay-uid"),
			}},
		},
	}
	assert.True(t, ownedByQuay(secret, quay))

	secret.OwnerReferences[0].UID = types.UID("other-uid")
	assert.False(t, ownedByQuay(secret, quay))

	secret.OwnerReferences[0].UID = ""
	assert.False(t, ownedByQuay(secret, quay))

	quay.UID = ""
	secret.OwnerReferences[0].UID = types.UID("quay-uid")
	assert.False(t, ownedByQuay(secret, quay))
}

func TestDeleteReadOnlySecretRequiresOwnership(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry",
			Namespace: "ns",
			UID:       types.UID("quay-uid"),
		},
	}

	unowned := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      readOnlySecretName(quay),
			Namespace: "ns",
		},
	}
	client := fake.NewClientBuilder().WithScheme(s).WithObjects(quay, unowned).Build()
	reconciler := &QuayRegistryReconciler{Client: client}

	err := reconciler.deleteReadOnlySecret(context.Background(), quay)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not owned by this QuayRegistry")

	var got corev1.Secret
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: unowned.Name, Namespace: unowned.Namespace}, &got))

	owned := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      readOnlySecretName(quay),
			Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "QuayRegistry",
				Name:       "registry",
				UID:        types.UID("quay-uid"),
			}},
		},
	}
	client = fake.NewClientBuilder().WithScheme(s).WithObjects(quay, owned).Build()
	reconciler = &QuayRegistryReconciler{Client: client}

	require.NoError(t, reconciler.deleteReadOnlySecret(context.Background(), quay))
	err = client.Get(context.Background(), types.NamespacedName{Name: owned.Name, Namespace: owned.Namespace}, &got)
	assert.True(t, errors.IsNotFound(err))
}

func TestConditionEventTypeWarnsForComponentCreationFailures(t *testing.T) {
	for _, reason := range []v1.ConditionReason{
		v1.ConditionReasonPostgresUpgradeFailed,
		v1.ConditionReasonMigrationsFailed,
		v1.ConditionReasonMigrationsJobMissing,
	} {
		t.Run(string(reason), func(t *testing.T) {
			assert.Equal(t, corev1.EventTypeWarning, conditionEventType(v1.ConditionComponentsCreated, metav1.ConditionFalse, reason))
		})
	}

	assert.Equal(
		t,
		corev1.EventTypeNormal,
		conditionEventType(v1.ConditionComponentsCreated, metav1.ConditionFalse, v1.ConditionReasonComponentsCreationSuccess),
	)
	assert.Equal(
		t,
		corev1.EventTypeWarning,
		conditionEventType(v1.ConditionTypeRolloutBlocked, metav1.ConditionTrue, v1.ConditionReasonComponentCreationFailed),
	)
}

func TestUpdateReadOnlyConditionSkipsUnchangedCondition(t *testing.T) {
	lastUpdate := metav1.NewTime(time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC))
	lastTransition := metav1.NewTime(time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC))
	quay := &v1.QuayRegistry{
		Status: v1.QuayRegistryStatus{
			LastUpdate: "unchanged",
			Conditions: []v1.Condition{{
				Type:               v1.ConditionTypeReadOnly,
				Status:             metav1.ConditionTrue,
				Reason:             v1.ConditionReasonReadOnlyActive,
				Message:            "operator-managed read-only mode is active",
				LastUpdateTime:     lastUpdate,
				LastTransitionTime: lastTransition,
			}},
		},
	}
	reconciler := &QuayRegistryReconciler{}

	err := reconciler.updateReadOnlyCondition(
		context.Background(),
		quay,
		metav1.ConditionTrue,
		v1.ConditionReasonReadOnlyActive,
		"operator-managed read-only mode is active",
	)

	require.NoError(t, err)
	assert.Equal(t, "unchanged", quay.Status.LastUpdate)
	condition := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeReadOnly)
	require.NotNil(t, condition)
	assert.Equal(t, lastUpdate, condition.LastUpdateTime)
	assert.Equal(t, lastTransition, condition.LastTransitionTime)
}

func TestReadOnlyActiveConditionMessageReportsDeferredUpgrade(t *testing.T) {
	originalVersion := v1.QuayVersionCurrent
	v1.QuayVersionCurrent = "v3.20.0"
	defer func() {
		v1.QuayVersionCurrent = originalVersion
	}()

	quay := &v1.QuayRegistry{
		Status: v1.QuayRegistryStatus{
			CurrentVersion: "v3.19.0",
		},
	}

	assert.Equal(
		t,
		"operator-managed read-only mode is active",
		readOnlyActiveConditionMessage(quay, &quaycontext.QuayRegistryContext{}),
	)

	assert.Equal(
		t,
		"operator upgrade from v3.19.0 to v3.20.0 deferred while operator-managed read-only mode is active",
		readOnlyActiveConditionMessage(quay, &quaycontext.QuayRegistryContext{ReadOnlyDeferUpgrade: true}),
	)

	quay.Status.CurrentVersion = "v3.20.0"
	assert.Equal(
		t,
		"operator-managed read-only mode is active",
		readOnlyActiveConditionMessage(quay, &quaycontext.QuayRegistryContext{ReadOnlyDeferUpgrade: true}),
	)
}

func TestReadOnlyOperatorConfigConflictDetectsFragments(t *testing.T) {
	source, err := yaml.Marshal(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"})
	require.NoError(t, err)
	fragment, err := yaml.Marshal(map[string]interface{}{"REGISTRY_STATE": "readonly"})
	require.NoError(t, err)
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml":        source,
			"mirror.config.yaml": fragment,
		},
	}

	assert.Equal(t, "mirror.config.yaml", readOnlyOperatorConfigConflict(secret))
}

func TestReadOnlyOverrideConflict(t *testing.T) {
	quay := &v1.QuayRegistry{
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{
					Kind:    v1.ComponentQuay,
					Managed: true,
					Overrides: &v1.Override{
						Env: []corev1.EnvVar{{
							Name:  "QUAY_OVERRIDE_CONFIG",
							Value: `{"REGISTRY_STATE":"normal"}`,
						}},
					},
				},
			},
		},
	}

	conflict := readOnlyOverrideConflict(quay)
	assert.Contains(t, conflict, "REGISTRY_STATE")
}

func TestReadOnlyKeyURLUsesInternalService(t *testing.T) {
	quay := &v1.QuayRegistry{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "ns"}}

	url := readOnlyKeyURL(quay, "kid")
	assert.True(t, strings.HasPrefix(url, "http://registry-quay-app.ns.svc:80/"))
	assert.Contains(t, url, "/keys/services/quay/keys/kid")
}

func TestEnsureReadOnlyHPAPin(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	replicas := int32(3)
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "ns"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: v1.ComponentHPA, Managed: true},
			},
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-quay-app", Namespace: "ns"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	min := int32(1)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-quay-app", Namespace: "ns"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Name: "registry-quay-app"},
			MinReplicas:    &min,
			MaxReplicas:    10,
		},
	}
	client := fake.NewClientBuilder().WithScheme(s).WithObjects(quay, dep, hpa).Build()
	reconciler := &QuayRegistryReconciler{Client: client}

	pinned, gotReplicas, err := reconciler.ensureReadOnlyHPAPin(context.Background(), quay, "registry-quay-app")
	require.NoError(t, err)
	assert.False(t, pinned)
	assert.Equal(t, replicas, gotReplicas)

	var got autoscalingv2.HorizontalPodAutoscaler
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "registry-quay-app", Namespace: "ns"}, &got))
	require.NotNil(t, got.Spec.MinReplicas)
	assert.Equal(t, replicas, *got.Spec.MinReplicas)
	assert.Equal(t, replicas, got.Spec.MaxReplicas)
	assert.Equal(t, "1", got.Annotations[readOnlyOriginalMinReplicasAnnotation])
	assert.Equal(t, "10", got.Annotations[readOnlyOriginalMaxReplicasAnnotation])
}

func TestManualReadOnlyConfiguredFromLiteralOverride(t *testing.T) {
	quay := &v1.QuayRegistry{
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{
					Kind:    v1.ComponentMirror,
					Managed: true,
					Overrides: &v1.Override{
						Env: []corev1.EnvVar{{
							Name:  "QUAY_OVERRIDE_CONFIG",
							Value: `{"REGISTRY_STATE":"readonly"}`,
						}},
					},
				},
			},
		},
	}
	config, err := yaml.Marshal(map[string]interface{}{"SERVER_HOSTNAME": "quay.io"})
	require.NoError(t, err)
	secret := &corev1.Secret{Data: map[string][]byte{"config.yaml": config}}

	assert.True(t, manualReadOnlyConfigured(map[string]interface{}{}, secret, quay))
}

func TestApplyReadOnlyIntentSetsContext(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "ns"},
		Status:     v1.QuayRegistryStatus{ReadOnlyKeyID: "kid"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{{Kind: v1.ComponentMirror, Managed: false}},
		},
	}
	qctx := &quaycontext.QuayRegistryContext{}
	reconciler := &QuayRegistryReconciler{Client: fake.NewClientBuilder().Build()}
	intent := readOnlyIntent{
		Enabled:              true,
		Phase:                v1.ReadOnlyPhaseReadOnly,
		MountServiceKey:      true,
		RenderReadOnlyConfig: true,
		DeferUpgrade:         true,
	}

	require.NoError(t, reconciler.applyReadOnlyIntent(context.Background(), quay, qctx, intent))
	assert.Equal(t, string(v1.ReadOnlyPhaseReadOnly), qctx.ReadOnlyPhase)
	assert.Equal(t, "kid", qctx.ReadOnlyKeyID)
	assert.Equal(t, "registry-readonly-service-key", qctx.ReadOnlySecretName)
	assert.True(t, qctx.ReadOnlyMountEnabled)
	assert.True(t, qctx.ReadOnlyRegistryState)
	assert.True(t, qctx.ReadOnlyDeferUpgrade)
}
