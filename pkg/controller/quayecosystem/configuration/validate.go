package configuration

import (
	"context"
	"fmt"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Validate validates the QuayEcosystem
func (r *ReconcileQuayEcosystemConfiguration) Validate(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) (bool, error) {

	if len(quayEcosystem.Spec.ImagePullSecretName) != 0 {
		imagePullSecretName := r.quayEcosystem.Spec.ImagePullSecretName
		imagePullSecret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: r.quayEcosystem.Namespace, Name: imagePullSecretName}, imagePullSecret)
		if err != nil && errors.IsNotFound(err) {
			logging.Log.Error(fmt.Errorf("Pull Secret not Found"), "Pull Secret Validation", "Namespace", r.quayEcosystem.Namespace, "Name", imagePullSecretName)
			return false, nil
		} else if err != nil && !errors.IsNotFound(err) {
			return false, err
		}
	}

	return true, nil
}
