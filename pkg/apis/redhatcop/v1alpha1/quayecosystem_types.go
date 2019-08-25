package v1alpha1

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuayEcosystemSpec defines the desired state of QuayEcosystem
// +k8s:openapi-gen=true
type QuayEcosystemSpec struct {
	Quay  *Quay  `json:"quay,omitempty"`
	Redis *Redis `json:"redis,omitempty"`
	Clair *Clair `json:"clair,omitempty"`
}

// QuayEcosystemPhase defines the phase of lifecycle the operator is running in
type QuayEcosystemPhase string

// QuayEcosystemConditionType defines the types of conditions the operator will run through
type QuayEcosystemConditionType string

const (

	// QuayEcosystemValidationFailure indicates that there was an error validating the configuration
	QuayEcosystemValidationFailure QuayEcosystemConditionType = "QuayEcosystemValidationFailure"

	// QuayEcosystemUpdateDefaultConfigurationConditionSuccess represents successfully updating of the default spec configuration
	QuayEcosystemUpdateDefaultConfigurationConditionSuccess QuayEcosystemConditionType = "UpdateDefaultConfigurationSuccess"

	// QuayEcosystemUpdateDefaultConfigurationConditionFailure represents failing to updating of the default spec configuration
	QuayEcosystemUpdateDefaultConfigurationConditionFailure QuayEcosystemConditionType = "UpdateDefaultConfigurationFailure"

	// QuayEcosystemProvisioningSuccess indicates that the QuayEcosystem provisioning was successful
	QuayEcosystemProvisioningSuccess QuayEcosystemConditionType = "QuayEcosystemProvisioningSuccess"

	// QuayEcosystemProvisioningFailure indicates that the QuayEcosystem provisioning failed
	QuayEcosystemProvisioningFailure QuayEcosystemConditionType = "QuayEcosystemProvisioningFailure"

	// QuayEcosystemQuaySetupSuccess indicates that the Quay setup process was successful
	QuayEcosystemQuaySetupSuccess QuayEcosystemConditionType = "QuaySetupSuccess"
	// QuayEcosystemQuaySetupFailure indicates that the Quay setup process failed
	QuayEcosystemQuaySetupFailure QuayEcosystemConditionType = "QuaySetupFailure"

	// QuayEcosystemClairConfigurationSuccess indicates that the Clair configuration process succeeded
	QuayEcosystemClairConfigurationSuccess QuayEcosystemConditionType = "QuayEcosystemClairConfigurationSuccess"
	// QuayEcosystemClairConfigurationFailure indicates that the Clair configuration process failed
	QuayEcosystemClairConfigurationFailure QuayEcosystemConditionType = "QuayEcosystemClairConfigurationFailure"

	// QuayEcosystemSecurityScannerConfigurationSuccess indicates that the security scanner was configured successfully
	QuayEcosystemSecurityScannerConfigurationSuccess QuayEcosystemConditionType = "QuayEcosystemSecurityScannerConfigurationSuccess"
	// QuayEcosystemSecurityScannerConfigurationFailure indicates that the security scanner configuration failed
	QuayEcosystemSecurityScannerConfigurationFailure QuayEcosystemConditionType = "QuayEcosystemSecurityScannerConfigurationFailure"
)

// QuayEcosystemStatus defines the observed state of QuayEcosystem
// +k8s:openapi-gen=true
type QuayEcosystemStatus struct {
	Message  string             `json:"message,omitempty"`
	Phase    QuayEcosystemPhase `json:"phase,omitempty"`
	Hostname string             `json:"hostname,omitempty"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions    []QuayEcosystemCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
	SetupComplete bool                     `json:"setupComplete,omitempty"`
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
	ConfigResources                corev1.ResourceRequirements   `json:"configResources,omitempty" protobuf:"bytes,2,opt,name=configResources"`
	ConfigRouteHost                string                        `json:"configRouteHost,omitempty"`
	ConfigSecretName               string                        `json:"configSecretName,omitempty"`
	Database                       *Database                     `json:"database,omitempty"`
	DeploymentStrategy             appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	EnableNodePortService          bool                          `json:"enableNodePortService,omitempty"`
	Image                          string                        `json:"image,omitempty"`
	ImagePullSecretName            string                        `json:"imagePullSecretName,omitempty"`
	KeepConfigDeployment           bool                          `json:"keepConfigDeployment,omitempty"`
	NodeSelector                   map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	RegistryBackends               []RegistryBackend             `json:"registryBackends,omitempty"`
	RegistryStorage                *RegistryStorage              `json:"registryStorage,omitempty"`
	Replicas                       *int32                        `json:"replicas,omitempty"`
	Resources                      corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	RouteHost                      string                        `json:"routeHost,omitempty"`
	SkipSetup                      bool                          `json:"skipSetup,omitempty"`
	SslCertificatesSecretName      string                        `json:"sslCertificatesSecretName,omitempty"`
	SuperuserCredentialsSecretName string                        `json:"superuserCredentialsSecretName,omitempty"`
}

// QuayEcosystemCondition defines a list of conditions that the object will transiton through
type QuayEcosystemCondition struct {
	LastTransitionTime metav1.Time                `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	LastUpdateTime     metav1.Time                `json:"lastUpdateTime,omitempty" protobuf:"bytes,3,opt,name=lastUpdateTime"`
	Message            string                     `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
	Reason             string                     `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	Status             corev1.ConditionStatus     `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/kubernetes/pkg/api/v1.ConditionStatus"`
	Type               QuayEcosystemConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=QuayEcosystemConditionType"`
}

// Redis defines the properies of a deployment of Redis
type Redis struct {
	DeploymentStrategy  appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	Hostname            string                        `json:"hostname,omitempty"`
	Image               string                        `json:"image,omitempty"`
	ImagePullSecretName string                        `json:"imagePullSecretName,omitempty"`
	NodeSelector        map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	Port                *int32                        `json:"port,omitempty"`
	Replicas            *int32                        `json:"replicas,omitempty"`
	Resources           corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

// Database defines a database that will be deployed to support a particular component
type Database struct {
	CPU                   string                        `json:"cpu,omitempty"`
	CredentialsSecretName string                        `json:"credentialsSecretName,omitempty"`
	DeploymentStrategy    appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	Image                 string                        `json:"image,omitempty"`
	ImagePullSecretName   string                        `json:"imagePullSecretName,omitempty"`
	Memory                string                        `json:"memory,omitempty"`
	NodeSelector          map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	Replicas              *int32                        `json:"replicas,omitempty"`
	Resources             corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	Server                string                        `json:"server,omitempty"`
	VolumeSize            string                        `json:"volumeSize,omitempty"`
}

// Clair defines the properties of a deployment of Clair
type Clair struct {
	Database                  *Database                     `json:"database,omitempty"`
	DeploymentStrategy        appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	Enabled                   bool                          `json:"enabled,omitempty"`
	Image                     string                        `json:"image,omitempty"`
	ImagePullSecretName       string                        `json:"imagePullSecretName,omitempty"`
	NodeSelector              map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	Replicas                  *int32                        `json:"replicas,omitempty"`
	Resources                 corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	SslCertificatesSecretName string                        `json:"sslCertificatesSecretName,omitempty"`
	UpdateInterval            string                        `json:"updateInterval,omitempty"`
}

// RegistryBackend defines a particular backend supporting the Quay registry
type RegistryBackend struct {
	Name                  string `json:"name"`
	RegistryBackendSource `json:",inline" protobuf:"bytes,2,opt,name=registryBackendSource"`
}

// RegistryBackendSource defines the specific configurations to support the Quay registry
type RegistryBackendSource struct {
	Local *LocalRegistryBackendSource `json:"local,omitempty,name=local"`
}

// RegistryStorage defines the configurations to support persistent storage
type RegistryStorage struct {
	PersistentVolumeAccessModes      []corev1.PersistentVolumeAccessMode `json:"persistentVolumeAccessMode,omitempty,name=persistentVolumeAccessMode"`
	PersistentVolumeSize             string                              `json:"persistentVolumeSize,omitempty,name=volumeSize"`
	PersistentVolumeStorageClassName string                              `json:"persistentVolumeStorageClassName,omitempty,name=storageClassName"`
}

// LocalRegistryBackendSource defines local registry storage
type LocalRegistryBackendSource struct {
	StoragePath string `json:"storage_path,omitempty,name=storage_path"`
}

func init() {
	SchemeBuilder.Register(&QuayEcosystem{}, &QuayEcosystemList{})
}

// SetCondition applies the condition
func (q *QuayEcosystem) SetCondition(newCondition QuayEcosystemCondition) *QuayEcosystemCondition {

	now := metav1.NewTime(time.Now())

	if q.Status.Conditions == nil {
		q.Status.Conditions = []QuayEcosystemCondition{}
	}

	existingCondition, found := q.FindConditionByType(newCondition.Type)

	if !found {
		newCondition.LastTransitionTime = now
		newCondition.LastUpdateTime = now

		q.Status.Conditions = append(q.Status.Conditions, newCondition)

		return &newCondition
	}

	if newCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = now
	}

	existingCondition.LastUpdateTime = now
	existingCondition.Message = newCondition.Message
	existingCondition.Reason = newCondition.Reason
	return existingCondition

}

// FindConditionByType locates the Condition by the type
func (q *QuayEcosystem) FindConditionByType(conditionType QuayEcosystemConditionType) (*QuayEcosystemCondition, bool) {

	for i := range q.Status.Conditions {
		if q.Status.Conditions[i].Type == conditionType {
			return &q.Status.Conditions[i], true
		}
	}

	return &QuayEcosystemCondition{}, false
}
