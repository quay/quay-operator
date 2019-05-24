package resources

import (
	"fmt"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/configuration/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewResourceObjectMeta builds ObjectMeta for all Kubernetes resources created by operator
func NewResourceObjectMeta(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      GetGenericResourcesName(quayEcosystem),
		Namespace: quayEcosystem.ObjectMeta.Namespace,
		Labels:    BuildResourceLabels(quayEcosystem),
	}
}

// GetGenericResourcesName returns name of Kubernetes resource name
func GetGenericResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return quayEcosystem.ObjectMeta.Name
}

// BuildResourceLabels returns labels for all Kubernetes resources created by operator
func BuildResourceLabels(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) map[string]string {
	return map[string]string{
		constants.LabelAppKey:    constants.LabelAppValue,
		constants.LabelQuayCRKey: quayEcosystem.Name,
	}
}

// BuildQuayResourceLabels builds labels for the Quay app resources
func BuildQuayResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentAppValue
	return resourceMap
}

// BuildQuayConfigResourceLabels builds labels for the Quay config resources
func BuildQuayConfigResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentConfigValue
	return resourceMap
}

// BuildQuayDatabaseResourceLabels builds labels for the Quay app resources
func BuildQuayDatabaseResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentQuayDatabaseValue
	return resourceMap
}

// BuildRedisResourceLabels builds labels for the Redis app resources
func BuildRedisResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentRedisValue
	return resourceMap
}

// GetQuayResourcesName returns name of Kubernetes resource name
func GetQuayResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay", GetGenericResourcesName(quayEcosystem))
}

// GetQuayConfigResourcesName returns name of Kubernetes resource name
func GetQuayConfigResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay-config", GetGenericResourcesName(quayEcosystem))
}

// GetRedisResourcesName returns name of Kubernetes resource name
func GetRedisResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-redis", GetGenericResourcesName(quayEcosystem))
}

// GetConfigMapSecretName returns the name of the Quay config secret
func GetConfigMapSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	//configSecretName := fmt.Sprintf("%s-config-secret", GetGenericResourcesName(quayEcosystem))
	return "quay-enterprise-config-secret"
	//return configSecretName
}

// GetQuayDatabaseName returns the name of the Quay database
func GetQuayDatabaseName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay-%s", GetGenericResourcesName(quayEcosystem), quayEcosystem.Spec.Quay.Database.Type)
}

// GetQuayRegistryStorageName returns the name of the Quay registry storage
func GetQuayRegistryStorageName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-registry", GetGenericResourcesName(quayEcosystem))
}

// UpdateMetaWithName updates the name of the resource
func UpdateMetaWithName(meta metav1.ObjectMeta, name string) metav1.ObjectMeta {
	meta.Name = name
	return meta
}
