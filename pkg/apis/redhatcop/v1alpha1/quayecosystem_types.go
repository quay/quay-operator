package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuayEcosystemSpec defines the desired state of QuayEcosystem
// +k8s:openapi-gen=true
type QuayEcosystemSpec struct {
	ImagePullSecretName string `json:"imagePullSecretName,omitempty"`
	Quay                Quay   `json:"quay,omitempty"`
	Redis               Redis  `json:"redis,omitempty"`
}

// DatabaseType defines type of database
type DatabaseType string

// QuayEcosystemPhase defines type of database
type QuayEcosystemPhase string

const (
	// QuayEcosystemPhaseValidationComplete indicates that validation of the Custom Resource was successfully completed
	QuayEcosystemPhaseValidationComplete QuayEcosystemPhase = "Validation Complete"
	// QuayEcosystemPhaseValidationError indicates there was an error validating the QuayEcosystem Custom Resource
	QuayEcosystemPhaseValidationError QuayEcosystemPhase = "Validation Error"
	// QuayEcosystemDeploymentError indicates there was an error deploying the Quay Ecosystem
	QuayEcosystemDeploymentError QuayEcosystemPhase = "Deployment Error"
	// DatabaseMySQL defines the name associated with the MySQL database instance
	DatabaseMySQL DatabaseType = "mysql"
	// DatabasePostgresql defines the name associated with the PostgreSQL database instance
	DatabasePostgresql DatabaseType = "postgresql"
)

// QuayEcosystemStatus defines the observed state of QuayEcosystem
// +k8s:openapi-gen=true
type QuayEcosystemStatus struct {
	Message string `json:"message,omitempty"`
	Phase   string `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayEcosystem is the Schema for the quayecosystems API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type QuayEcosystem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuayEcosystemSpec   `json:"spec,omitempty"`
	Status QuayEcosystemStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayEcosystemList contains a list of QuayEcosystem
type QuayEcosystemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuayEcosystem `json:"items"`
}

// Quay defines the properies of a deployment of Quay
type Quay struct {
	EnableNodePortService bool            `json:"enableNodePortService,omitempty"`
	Image                 string          `json:"image,omitempty"`
	RouteHost             string          `json:"routeHost,omitempty"`
	ConfigRouteHost       string          `json:"configRouteHost,omitempty"`
	Replicas              *int32          `json:"replicas,omitempty"`
	Database              Database        `json:"database,omitempty"`
	RegistryStorage       RegistryStorage `json:"registryStorage,omitempty"`
}

// Redis defines the properies of a deployment of Redis
type Redis struct {
	Skip     bool   `json:"skip,omitempty"`
	Image    string `json:"image,omitempty"`
	Replicas *int32 `json:"replicas,omitempty"`
}

// Database defines a database that will be deployed to support a particular component
type Database struct {
	Type                  DatabaseType `json:"type,omitempty"`
	Image                 string       `json:"image,omitempty"`
	VolumeSize            string       `json:"volumeSize,omitempty"`
	Memory                string       `json:"memory,omitempty"`
	CPU                   string       `json:"cpu,omitempty"`
	CredentialsSecretName string       `json:"credentialsSecretName,omitempty"`
}

type RegistryStorage struct {
	RegistryStorageType `json:",inline" protobuf:"bytes,2,opt,name=registryStorageType"`
	StorageDirectory    string `json:"storageDirectory,omitempty"`
}

type RegistryStorageType struct {
	PersistentVolume PersistentVolumeRegistryStorageType `json:"persistentVolume,omitempty"`
}

type PersistentVolumeRegistryStorageType struct {
	StorageClassName string                              `json:"storageClassName,omitempty"`
	Capacity         string                              `json:"capacity,omitempty"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

func init() {
	SchemeBuilder.Register(&QuayEcosystem{}, &QuayEcosystemList{})
}
