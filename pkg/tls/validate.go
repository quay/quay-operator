package tls

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchAndValidate fetches a TLS Secret and validates it contains non-empty tls.crt and tls.key.
// It returns the cert and key bytes along with the Secret object for further use (e.g. labeling).
func FetchAndValidate(ctx context.Context, cli client.Client, namespace, name string) (cert, key []byte, secret *corev1.Secret, err error) {
	nsn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	var s corev1.Secret
	if err := cli.Get(ctx, nsn, &s); err != nil {
		return nil, nil, nil, fmt.Errorf("unable to get TLS secret %q: %w", name, err)
	}

	cert, certOK := s.Data["tls.crt"]
	key, keyOK := s.Data["tls.key"]
	if !certOK || len(cert) == 0 {
		return nil, nil, nil, fmt.Errorf("TLS secret %q missing or empty tls.crt", name)
	}
	if !keyOK || len(key) == 0 {
		return nil, nil, nil, fmt.Errorf("TLS secret %q missing or empty tls.key", name)
	}

	return cert, key, &s, nil
}
