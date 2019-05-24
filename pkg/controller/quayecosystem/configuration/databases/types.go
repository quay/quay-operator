package databases

import (
	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Database represents a contract for a persistent store
type Database interface {
	GenerateResources(meta metav1.ObjectMeta, quayEcosystem *redhatcopv1alpha1.QuayEcosystem, database DatabaseConfig) ([]metav1.Object, error)
	ValidateProvidedSecret(secret *corev1.Secret) bool
	GetDefaultSecret(metav1.ObjectMeta, map[string]string) *corev1.Secret
}

// DatabaseConfig is an internal structure representing a database
type DatabaseConfig struct {
	Name            string
	Image           string
	Database        string
	Username        string
	Password        string
	RootPassword    string
	LimitsCPU       string
	LimitsMemory    string
	RequestsCPU     string
	RequestsMemory  string
	VolumeSize      string
	CredentialsName string
}
