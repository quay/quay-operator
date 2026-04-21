package cmpstatus

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
	quaytls "github.com/quay/quay-operator/pkg/tls"
)

// TLS checks a quay registry TLS status.
type TLS struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (t *TLS) Name() string {
	return "tls"
}

// Check verifies the status for a TLS component. If TLS is managed we expect not to find an entry
// for ssl keys in the config bundle secret while if TLS is unmanaged we do expect to find this
// entry.
func (t *TLS) Check(ctx context.Context, reg qv1.QuayRegistry) (qv1.Condition, error) {
	var zero qv1.Condition

	if reg.Spec.ConfigBundleSecret == "" {
		return qv1.Condition{
			Type:           qv1.ComponentTLSReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        "Config bundle secret not populated",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	nsn := types.NamespacedName{
		Namespace: reg.Namespace,
		Name:      reg.Spec.ConfigBundleSecret,
	}

	var secret corev1.Secret
	if err := t.Client.Get(ctx, nsn, &secret); err != nil {
		if errors.IsNotFound(err) {
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Config bundle does not exist",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		return zero, err
	}

	// External TLS secret mode: secretRef is set, validate the referenced secret.
	// Conflict checks mirror controllers/quay/features.go so status accurately
	// reflects reconcile-blocking conflicts.
	if reg.Spec.TLS != nil && reg.Spec.TLS.SecretRef != nil {
		if qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentTLS) {
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "spec.tls.secretRef cannot be used when the TLS component is managed",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		if _, hasCert := secret.Data["ssl.cert"]; hasCert {
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "spec.tls.secretRef and ssl.cert in configBundleSecret are mutually exclusive",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}
		if _, hasKey := secret.Data["ssl.key"]; hasKey {
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "spec.tls.secretRef and ssl.key in configBundleSecret are mutually exclusive",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		_, _, _, err := quaytls.FetchAndValidate(ctx, t.Client, reg.Namespace, reg.Spec.TLS.SecretRef.Name)
		if err != nil {
			msg := err.Error()
			if errors.IsNotFound(err) {
				msg = "External TLS secret not found"
			}
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        msg,
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentTLSReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentReady,
			Message:        "Using externally managed TLS certificate",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	_, hasCRT := secret.Data["ssl.cert"]
	_, hasKey := secret.Data["ssl.key"]

	// if tls is managed we do not expect to find entries for ssl.key and ssl.cert in the
	// config bundle secret.
	if qv1.ComponentIsManaged(reg.Spec.Components, qv1.ComponentTLS) {
		if hasCRT || hasKey {
			return qv1.Condition{
				Type:           qv1.ComponentTLSReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "TLS managed but config bundle contain certs",
				LastUpdateTime: metav1.NewTime(time.Now()),
			}, nil
		}

		return qv1.Condition{
			Type:           qv1.ComponentTLSReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentReady,
			Message:        "Using cluster wildcard certs",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	if !hasCRT || !hasKey {
		return qv1.Condition{
			Type:           qv1.ComponentTLSReady,
			Status:         metav1.ConditionFalse,
			Reason:         qv1.ConditionReasonComponentNotReady,
			Message:        "TLS unmanaged but config bundle does not contain certs",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return qv1.Condition{
		Type:           qv1.ComponentTLSReady,
		Status:         metav1.ConditionTrue,
		Reason:         qv1.ConditionReasonComponentReady,
		Message:        "Config bundle contains certs",
		LastUpdateTime: metav1.NewTime(time.Now()),
	}, nil
}
