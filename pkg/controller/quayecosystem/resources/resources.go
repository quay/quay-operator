package resources

import (
	"fmt"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
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

// BuildQuayRepoMirrorResourceLabels builds labels for the Quay repomirror resources
func BuildQuayRepoMirrorResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentRepoMirrorValue
	return resourceMap
}

// BuildClairResourceLabels builds labels for the Clair resources
func BuildClairResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentClairValue
	return resourceMap
}

// BuildQuayDatabaseResourceLabels builds labels for the Quay app resources
func BuildQuayDatabaseResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentQuayDatabaseValue
	return resourceMap
}

// BuildClairDatabaseResourceLabels builds labels for the Quay app resources
func BuildClairDatabaseResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentClairDatabaseValue
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

// GetQuayRepoMirrorResourcesName returns the name of the Quay repomirror resources
func GetQuayRepoMirrorResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay-repomirror", GetGenericResourcesName(quayEcosystem))
}

// GetClairResourcesName returns name of Clair app name
func GetClairResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair", GetGenericResourcesName(quayEcosystem))
}

// GetRedisResourcesName returns name of Kubernetes resource name
func GetRedisResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-redis", GetGenericResourcesName(quayEcosystem))
}

// GetQuayConfigMapSecretName returns the name of the Quay config secret
func GetQuayConfigMapSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	//configSecretName := fmt.Sprintf("%s-config-secret", GetGenericResourcesName(quayEcosystem))
	return "quay-enterprise-config-secret"
	//return configSecretName
}

// GetClairConfigMapName returns the name of the Clair configuration
func GetClairConfigMapName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair-config", GetGenericResourcesName(quayEcosystem))
}

// GetClairSecretName returns the name of the Clair secret
func GetClairSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair-secret", GetGenericResourcesName(quayEcosystem))
}

// GetClairEndpointAddress returns the URL of the Clair endpoint
func GetClairEndpointAddress(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("https://%s.%s.svc:%s", GetClairResourcesName(quayEcosystem), quayEcosystem.Namespace, constants.ClairPort)
}

// GetClairSSLSecretName returns the name of the secret containing the SSL certificate
func GetClairSSLSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair-ssl", GetGenericResourcesName(quayEcosystem))
}

// GetQuayExtraCertsSecretName returns the name of the Quay extra certs secret
func GetQuayExtraCertsSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return "quay-enterprise-cert-secret"
}

// GetDatabaseResourceName returns the name of the database
func GetDatabaseResourceName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem, databaseComponent constants.DatabaseComponent) string {
	return fmt.Sprintf("%s-%s-%s", GetGenericResourcesName(quayEcosystem), string(databaseComponent), constants.PostgresqlName)
}

// GetQuayRegistryStorageName returns the name of the Quay registry storage
func GetQuayRegistryStorageName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-registry", GetGenericResourcesName(quayEcosystem))
}

// GetRegistryStorageVolumeName returns the name that should be applied to the volume for the storage backend
func GetRegistryStorageVolumeName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem, registryBackendName string) string {
	return fmt.Sprintf("%s-%s", GetGenericResourcesName(quayEcosystem), registryBackendName)
}

// GetSecurityScannerSecretName returns the name of the secret containing the security scanner secret
func GetSecurityScannerSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-security-scanner", GetGenericResourcesName(quayEcosystem))
}

// GetSecurityScannerKeyNotes returns the notes that will be included with the security scanner key
func GetSecurityScannerKeyNotes(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("Created by Quay Operator %s", GetGenericResourcesName(quayEcosystem))
}

// UpdateMetaWithName updates the name of the resource
func UpdateMetaWithName(meta metav1.ObjectMeta, name string) metav1.ObjectMeta {
	meta.Name = name
	return meta
}

// GenerateClairCertificateSANs generates the SANs for the generated certificate
func GenerateClairCertificateSANs(serviceName, namespace string) []string {
	return []string{
		fmt.Sprintf("%s", serviceName),
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.local", serviceName, namespace),
	}
}
