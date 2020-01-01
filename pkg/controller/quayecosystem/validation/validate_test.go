package validation

import (
	"testing"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDefaultValidation(t *testing.T) {

	// Objects to track in the fake client.
	objs := []runtime.Object{}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
		IsOpenShift:   true,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	validate, err := Validate(cl, &quayConfiguration)

	assert.NoError(t, err)
	assert.Equal(t, validate, true)

}

func TestShortQuayPwdError(t *testing.T) {

	// Objects to track in the fake client.
	objs := []runtime.Object{}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	quayConfiguration.QuaySuperuserPassword = "ToShort"

	validate, err := Validate(cl, &quayConfiguration)

	assert.Error(t, err)
	assert.Equal(t, validate, false)

}

func TestValidateSecretError(t *testing.T) {

	// Objects to track in the fake client.
	objs := []runtime.Object{}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	// Set default values
	validQuaySuperuserSecret, superuserSecret, err := validateSecret(cl, quayConfiguration.QuayEcosystem.Namespace, quayConfiguration.QuayEcosystem.Spec.Quay.SuperuserCredentialsSecretName, constants.DefaultQuaySuperuserCredentials)

	assert.Error(t, err)
	assert.Equal(t, validQuaySuperuserSecret, false)
	assert.Empty(t, superuserSecret)
}

func TestValidateDefaultSecret(t *testing.T) {
	// Stub a secret resource object
	data := make(map[string][]byte)
	data["superuser-username"] = []byte("quay")
	data["superuser-password"] = []byte("password")
	data["superuser-email"] = []byte("quay@redhat.com")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "SecretTest",
			Namespace: "SecretTest",
		},
		Data: data,
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		secret,
	}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	quayConfiguration.QuayEcosystem.Namespace = "SecretTest"
	quayConfiguration.QuayEcosystem.Spec.Quay.SuperuserCredentialsSecretName = "SecretTest"

	// Test validate Secret function
	validQuaySuperuserSecret, superuserSecret, err := validateSecret(cl, quayConfiguration.QuayEcosystem.Namespace, quayConfiguration.QuayEcosystem.Spec.Quay.SuperuserCredentialsSecretName, constants.DefaultQuaySuperuserCredentials)

	assert.NoError(t, err)
	assert.Equal(t, validQuaySuperuserSecret, true)
	assert.Equal(t, superuserSecret, secret)
}

func TestValidateRedisSecret(t *testing.T) {
	// Stub a secret resource object
	data := make(map[string][]byte)
	data["superuser-username"] = []byte("quay")
	data["superuser-password"] = []byte("password")
	data["superuser-email"] = []byte("quay@redhat.com")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "SecretTest",
			Namespace: "SecretTest",
		},
		Data: data,
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		secret,
	}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	quayConfiguration.QuayEcosystem.Namespace = "SecretTest"
	// Redis Image Name
	quayConfiguration.QuayEcosystem.Spec.Redis.ImagePullSecretName = "SecretTest"

	// Test validate Secret function
	validQuaySuperuserSecret, superuserSecret, err := validateSecret(cl, quayConfiguration.QuayEcosystem.Namespace, quayConfiguration.QuayEcosystem.Spec.Quay.SuperuserCredentialsSecretName, nil)

	assert.NoError(t, err)
	assert.Equal(t, validQuaySuperuserSecret, true)
	assert.Equal(t, superuserSecret, secret)
}

func TestValidateQuayDatabaseSecret(t *testing.T) {
	// Stub a secret resource object
	data := make(map[string][]byte)
	data["superuser-username"] = []byte("quay")
	data["superuser-password"] = []byte("password")
	data["superuser-email"] = []byte("quay@redhat.com")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "SecretTest",
			Namespace: "SecretTest",
		},
		Data: data,
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		secret,
	}
	// Initialize fake client
	cl := fake.NewFakeClient(objs...)
	// Stub out object placeholders for test
	quayEcosystem := &redhatcopv1alpha1.QuayEcosystem{}
	quayConfiguration := resources.QuayConfiguration{
		QuayEcosystem: quayEcosystem,
	}

	// Set default values
	defaultConfig := SetDefaults(cl, &quayConfiguration)
	assert.Equal(t, defaultConfig, true)

	quayConfiguration.QuayEcosystem.Namespace = "SecretTest"
	// Quay Database Image Name
	quayConfiguration.QuayEcosystem.Spec.Quay.Database.ImagePullSecretName = "SecretTest"

	// Test validate Secret function
	validQuaySuperuserSecret, superuserSecret, err := validateSecret(cl, quayConfiguration.QuayEcosystem.Namespace, quayConfiguration.QuayEcosystem.Spec.Quay.Database.ImagePullSecretName, nil)

	assert.NoError(t, err)
	assert.Equal(t, validQuaySuperuserSecret, true)
	assert.Equal(t, superuserSecret, secret)
}
