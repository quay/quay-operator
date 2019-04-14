package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuayEcosystemSpec defines the desired state of QuayEcosystem
type QuayEcosystemSpec struct {
	ImagePullSecretName string `json:"imagePullSecretName,omitempty"`
	Quay                Quay   `json:"quay,omitempty"`
	Redis               Redis  `json:"redis,omitempty"`
}

// QuayEcosystemStatus defines the observed state of QuayEcosystem
type QuayEcosystemStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayEcosystem is the Schema for the quayecosystems API
// +k8s:openapi-gen=true
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

// DatabaseType defines type of database
type DatabaseType string

const (
	// DatabaseMySQL defines the name associated with the MySQL database instance
	DatabaseMySQL DatabaseType = "mysql"
	// DatabasePostgresql defines the name associated with the PostgreSQL database instance
	DatabasePostgresql DatabaseType = "postgresql"
)

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
