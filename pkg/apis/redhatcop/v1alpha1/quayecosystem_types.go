package v1alpha1

import (
	"time"

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
	EnableNodePortService          bool            `json:"enableNodePortService,omitempty"`
	Image                          string          `json:"image,omitempty"`
	RouteHost                      string          `json:"routeHost,omitempty"`
	ConfigRouteHost                string          `json:"configRouteHost,omitempty"`
	Replicas                       *int32          `json:"replicas,omitempty"`
	Database                       Database        `json:"database,omitempty"`
	RegistryStorage                RegistryStorage `json:"registryStorage,omitempty"`
	SkipSetup                      bool            `json:"skipSetup,omitempty"`
	KeepConfigDeployment           bool            `json:"keepConfigDeployment,omitempty"`
	IsOpenShift                    bool            `json:"isOpenShift,omitempty"`
	SuperuserCredentialsSecretName string          `json:"superuserCredentialsName,omitempty"`
	ConfigSecretName               string          `json:"configSecretName,omitempty"`
}

// QuayEcosystemCondition defines a list of conditions that the object will transiton through
type QuayEcosystemCondition struct {
	Type               QuayEcosystemConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=QuayEcosystemConditionType"`
	Status             corev1.ConditionStatus     `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/kubernetes/pkg/api/v1.ConditionStatus"`
	LastUpdateTime     metav1.Time                `json:"lastUpdateTime,omitempty" protobuf:"bytes,3,opt,name=lastUpdateTime"`
	LastTransitionTime metav1.Time                `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	Reason             string                     `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	Message            string                     `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

// Redis defines the properies of a deployment of Redis
type Redis struct {
	Image    string `json:"image,omitempty"`
	Replicas *int32 `json:"replicas,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Port     *int32 `json:"port,omitempty"`
}

// Database defines a database that will be deployed to support a particular component
type Database struct {
	Image                 string `json:"image,omitempty"`
	VolumeSize            string `json:"volumeSize,omitempty"`
	Memory                string `json:"memory,omitempty"`
	CPU                   string `json:"cpu,omitempty"`
	Replicas              *int32 `json:"replicas,omitempty"`
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
	Server                string `json:"server,omitempty"`
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

func (q *QuayEcosystem) FindConditionByType(conditionType QuayEcosystemConditionType) (*QuayEcosystemCondition, bool) {

	for i := range q.Status.Conditions {
		if q.Status.Conditions[i].Type == conditionType {
			return &q.Status.Conditions[i], true
		}
	}

	return &QuayEcosystemCondition{}, false
}
