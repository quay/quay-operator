package cmpstatus

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

func TestTLSCheck(t *testing.T) {
	for _, tt := range []struct {
		name string
		quay qv1.QuayRegistry
		objs []runtime.Object
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
			objs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.key": []byte(""),
						"ssl.crt": []byte(""),
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
			objs: []runtime.Object{
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
			objs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-bundle",
					},
					Data: map[string][]byte{
						"ssl.key": []byte(""),
						"ssl.crt": []byte(""),
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
			objs: []runtime.Object{
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

			cli := fake.NewFakeClient(tt.objs...)
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
