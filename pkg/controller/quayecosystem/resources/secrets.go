package resources

import (
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetSecretDefinition(meta metav1.ObjectMeta) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Data:       map[string][]byte{},
	}
}

func GetSecretDefinitionFromCredentialsMap(name string, meta metav1.ObjectMeta, secretMap map[string]string) *corev1.Secret {

	meta.Name = name

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		StringData: secretMap,
	}
}

func GetTLSSecretDefinition(meta metav1.ObjectMeta, tlsPrivateKey []byte, tlsCertKey []byte) *corev1.Secret {

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: meta,
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: tlsPrivateKey,
			corev1.TLSCertKey:       tlsCertKey,
		},
	}
}
