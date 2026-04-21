package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestTLSCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []client.Object
		cond qv1.Condition
	}{
		{
			name: "unmanaged but config bundle does not exist",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "does-not-exist",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Config bundle does not exist",
			},
		},
		{
			name: "unmanaged without config bundle secret",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Config bundle secret not populated",
			},
		},
		{
			name: "managed tls with extra certs",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.key":  []byte(""),
						"ssl.cert": []byte(""),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "TLS managed but config bundle contain certs",
			},
		},
		{
			name: "managed tls without extra certs",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"config.yaml": []byte(""),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Using cluster wildcard certs",
			},
		},
		{
			name: "unmanaged tls with certs",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.key":  []byte(""),
						"ssl.cert": []byte(""),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Config bundle contains certs",
			},
		},
		{
			name: "secretRef with managed TLS component",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: true,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "spec.tls.secretRef cannot be used when the TLS component is managed",
			},
		},
		{
			name: "secretRef with ssl.cert in config bundle",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.cert": []byte("cert-data"),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "spec.tls.secretRef and ssl.cert in configBundleSecret are mutually exclusive",
			},
		},
		{
			name: "secretRef with ssl.key in config bundle",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.key": []byte("key-data"),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "spec.tls.secretRef and ssl.key in configBundleSecret are mutually exclusive",
			},
		},
		{
			name: "secretRef with valid external secret",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-tls",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionTrue,
				Reason:  qv1.ConditionReasonComponentReady,
				Message: "Using externally managed TLS certificate",
			},
		},
		{
			name: "secretRef with missing external secret",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "missing"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "External TLS secret not found",
			},
		},
		{
			name: "secretRef with missing tls.key in external secret",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					TLS: &qv1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-tls",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "TLS secret \"my-tls\" missing or empty tls.key",
			},
		},
		{
			name: "unmanaged tls without certs",
			quay: qv1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "registry",
					UID:  "uid",
				},
				Spec: qv1.QuayRegistrySpec{
					ConfigBundleSecret: "config-bundle",
					Components: []qv1.Component{
						{
							Kind:    qv1.ComponentTLS,
							Managed: false,
						},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"foo": []byte(""),
					},
				},
			},
			cond: qv1.Condition{
				Type:    qv1.ComponentTLSReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "TLS unmanaged but config bundle does not contain certs",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			tls := TLS{cli}

			cond, err := tls.Check(ctx, tt.quay)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if cond.LastUpdateTime.IsZero() {
				t.Errorf("unexpected zeroed last update time for condition")
			}

			cond.LastUpdateTime = metav1.NewTime(time.Time{})
			if !reflect.DeepEqual(tt.cond, cond) {
				t.Errorf("expecting %+v, received %+v", tt.cond, cond)
			}
		})
	}
}
