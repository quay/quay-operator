package validation

import (
	"testing"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDefaultConfiguration(t *testing.T) {

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

	// Test for the expected default values
	assert.Equal(t, defaultConfig, true)
	assert.Equal(t, constants.QuayConfigUsername, quayConfiguration.QuayConfigUsername)
	assert.Equal(t, constants.QuayConfigDefaultPasswordValue, quayConfiguration.QuayConfigPassword)
	assert.Equal(t, constants.QuaySuperuserDefaultUsername, quayConfiguration.QuaySuperuserUsername)
	assert.Equal(t, constants.QuaySuperuserDefaultPassword, quayConfiguration.QuaySuperuserPassword)
	assert.Equal(t, constants.QuayImage, quayConfiguration.QuayEcosystem.Spec.Quay.Image)
	assert.Equal(t, constants.RedisImage, quayConfiguration.QuayEcosystem.Spec.Redis.Image)
	assert.Equal(t, constants.PostgresqlImage, quayConfiguration.QuayEcosystem.Spec.Quay.Database.Image)
	assert.Equal(t, true, quayConfiguration.DeployQuayConfiguration)
	assert.Equal(t, constants.QuayRegistryStoragePersistentVolumeAccessModes, quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeAccessModes)
	assert.Equal(t, constants.QuayRegistryStoragePersistentVolumeStoreSize, quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeSize)

	registry := []redhatcopv1alpha1.RegistryBackend{
		redhatcopv1alpha1.RegistryBackend{
			Name: constants.RegistryStorageDefaultName,
			RegistryBackendSource: redhatcopv1alpha1.RegistryBackendSource{
				Local: &redhatcopv1alpha1.LocalRegistryBackendSource{
					StoragePath: constants.QuayRegistryStoragePath,
				},
			},
		},
	}
	assert.Equal(t, registry, quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends)

}
