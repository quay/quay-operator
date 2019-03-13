package databases

import (
	copv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/cop/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Database represents a contract for a persistent store
type Database interface {
	GenerateResources(meta metav1.ObjectMeta, quatEcosystem *copv1alpha1.QuayEcosystem, database DatabaseConfig) ([]metav1.Object, error)
}

// DatabaseConfig is an internal structure representing a database
type DatabaseConfig struct {
	Name           string
	Image          string
	Database       string
	Username       string
	Password       string
	RootPassword   string
	LimitsCPU      string
	LimitsMemory   string
	RequestsCPU    string
	RequestsMemory string
	VolumeSize     string
}
