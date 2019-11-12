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
	ConfigEnvVars                  []corev1.EnvVar               `json:"configEnvVars,omitempty"`
	ConfigResources                corev1.ResourceRequirements   `json:"configResources,omitempty" protobuf:"bytes,2,opt,name=configResources"`
	ConfigRouteHost                string                        `json:"configRouteHost,omitempty"`
	ConfigSecretName               string                        `json:"configSecretName,omitempty"`
	Database                       *Database                     `json:"database,omitempty"`
	DeploymentStrategy             appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	EnableNodePortService          bool                          `json:"enableNodePortService,omitempty"`
	EnvVars                        []corev1.EnvVar               `json:"envVars,omitempty"`
	Image                          string                        `json:"image,omitempty"`
	ImagePullSecretName            string                        `json:"imagePullSecretName,omitempty"`
	LivenessProbe                  *corev1.Probe                 `json:"livenessProbe,omitempty"`
	KeepConfigDeployment           bool                          `json:"keepConfigDeployment,omitempty"`
	NodeSelector                   map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	ReadinessProbe                 *corev1.Probe                 `json:"readinessProbe,omitempty"`
	RegistryBackends               []RegistryBackend             `json:"registryBackends,omitempty"`
	RegistryStorage                *RegistryStorage              `json:"registryStorage,omitempty"`
	Replicas                       *int32                        `json:"replicas,omitempty"`
	Resources                      corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	RouteHost                      string                        `json:"routeHost,omitempty"`
	SkipSetup                      bool                          `json:"skipSetup,omitempty"`
	SslCertificatesSecretName      string                        `json:"sslCertificatesSecretName,omitempty"`
	SuperuserCredentialsSecretName string                        `json:"superuserCredentialsSecretName,omitempty"`
	EnableStorageReplication       bool                          `json:"enableStorageReplication,omitempty"`
	ExtraCaCerts                   []ExtraCaCert                 `json:"extraCaCerts,omitempty" patchStrategy:"merge" patchMergeKey:"secretName" protobuf:"bytes,2,rep,name=extraCaCerts"`
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
	CredentialsSecretName string                        `json:"credentialsSecretName,omitempty"`
	DeploymentStrategy    appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	EnvVars               []corev1.EnvVar               `json:"envVars,omitempty"`
	Hostname              string                        `json:"hostname,omitempty"`
	Image                 string                        `json:"image,omitempty"`
	ImagePullSecretName   string                        `json:"imagePullSecretName,omitempty"`
	LivenessProbe         *corev1.Probe                 `json:"livenessProbe,omitempty"`
	NodeSelector          map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	Port                  *int32                        `json:"port,omitempty"`
	ReadinessProbe        *corev1.Probe                 `json:"readinessProbe,omitempty"`
	Replicas              *int32                        `json:"replicas,omitempty"`
	Resources             corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

// Database defines a database that will be deployed to support a particular component
type Database struct {
	CPU                   string                        `json:"cpu,omitempty"`
	CredentialsSecretName string                        `json:"credentialsSecretName,omitempty"`
	DeploymentStrategy    appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	EnvVars               []corev1.EnvVar               `json:"envVars,omitempty"`
	Image                 string                        `json:"image,omitempty"`
	ImagePullSecretName   string                        `json:"imagePullSecretName,omitempty"`
	LivenessProbe         *corev1.Probe                 `json:"livenessProbe,omitempty"`
	Memory                string                        `json:"memory,omitempty"`
	NodeSelector          map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	ReadinessProbe        *corev1.Probe                 `json:"readinessProbe,omitempty"`
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
	EnvVars                   []corev1.EnvVar               `json:"envVars,omitempty"`
	Image                     string                        `json:"image,omitempty"`
	ImagePullSecretName       string                        `json:"imagePullSecretName,omitempty"`
	LivenessProbe             *corev1.Probe                 `json:"livenessProbe,omitempty"`
	NodeSelector              map[string]string             `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	ReadinessProbe            *corev1.Probe                 `json:"readinessProbe,omitempty"`
	Replicas                  *int32                        `json:"replicas,omitempty"`
	Resources                 corev1.ResourceRequirements   `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	SslCertificatesSecretName string                        `json:"sslCertificatesSecretName,omitempty"`
	UpdateInterval            string                        `json:"updateInterval,omitempty"`
}

// RegistryBackend defines a particular backend supporting the Quay registry
type RegistryBackend struct {
	Name                  string `json:"name"`
	RegistryBackendSource `json:",inline" protobuf:"bytes,2,opt,name=registryBackendSource"`
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
	ReplicateByDefault    *bool  `json:"replicateByDefault,omitempty"`
}

// RegistryBackendStorageType defines the type of registry backend storage
type RegistryBackendStorageType interface {
	Validate() error
}

// RegistryBackendSource defines the specific configurations to support the Quay registry
type RegistryBackendSource struct {
	Local       *LocalRegistryBackendSource       `json:"local,omitempty,name=local"`
	S3          *S3RegistryBackendSource          `json:"s3,omitempty,name=s3"`
	GoogleCloud *GoogleCloudRegistryBackendSource `json:"googlecloud,omitempty,name=googlecloud"`
	Azure       *AzureRegistryBackendSource       `json:"azure,omitempty,name=azure"`
	RADOS       *RADOSRegistryBackendSource       `json:"rados,omitempty,name=rados"`
	RHOCS       *RHOCSRegistryBackendSource       `json:"rhocs,omitempty,name=rhocs"`
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

// S3RegistryBackendSource defines S3 registry storage
type S3RegistryBackendSource struct {
	StoragePath string `json:"storage_path,omitempty,name=storage_path"`
	BucketName  string `json:"s3_bucket,omitempty,name=s3_bucket"`
	AccessKey   string `json:"s3_access_key,omitempty,name=s3_access_key"`
	SecretKey   string `json:"s3_secret_key,omitempty,name=s3_secret_key"`
	Host        string `json:"host,omitempty,name=host"`
	Port        int    `json:"port,omitempty,name=port"`
}

// GoogleCloudRegistryBackendSource defines Google Cloud registry storage
type GoogleCloudRegistryBackendSource struct {
	StoragePath string `json:"storage_path,omitempty,name=storage_path"`
	BucketName  string `json:"bucket_name,omitempty,name=bucket_name"`
	AccessKey   string `json:"access_key,omitempty,name=access_key"`
	SecretKey   string `json:"secret_key,omitempty,name=secret_key"`
}

// AzureRegistryBackendSource defines Azure blob registry storage
type AzureRegistryBackendSource struct {
	StoragePath   string `json:"storage_path,omitempty,name=storage_path"`
	ContainerName string `json:"azure_container,omitempty,name=azure_container"`
	AccountName   string `json:"azure_account_name,omitempty,name=azure_account_name"`
	AccountKey    string `json:"azure_account_key,omitempty,name=azure_account_key"`
	SasToken      string `json:"sas_token,omitempty,name=sas_token"`
}

// RADOSRegistryBackendSource defines Ceph RADOS registry storage
type RADOSRegistryBackendSource struct {
	StoragePath string `json:"storage_path,omitempty,name=storage_path"`
	BucketName  string `json:"s3_bucket,omitempty,name=s3_bucket"`
	AccessKey   string `json:"access_key,omitempty,name=s3_access_key"`
	SecretKey   string `json:"secret_key,omitempty,name=s3_secret_key"`
	Hostname    string `json:"hostname,omitempty,name=hostname"`
	Secure      bool   `json:"is_secure,omitempty,name=is_secure"`
	Port        int    `json:"port,omitempty,name=port"`
}

// RHOCSRegistryBackendSource defines RHOCS registry storage
type RHOCSRegistryBackendSource struct {
	StoragePath string `json:"storage_path,omitempty,name=storage_path"`
	BucketName  string `json:"bucket_name,omitempty,name=bucket_name"`
	AccessKey   string `json:"access_key,omitempty,name=access_key"`
	SecretKey   string `json:"secret_key,omitempty,name=access_key"`
	Hostname    string `json:"hostname,omitempty,name=hostname"`
	Secure      bool   `json:"is_secure,omitempty,name=is_secure"`
	Port        int    `json:"port,omitempty,name=port"`
}

// Not yet implemented

// SwiftRegistryBackendSource defines Swift registry storage
type SwiftRegistryBackendSource struct {
	AuthVersion int               `json:"auth_version,omitempty,name=auth_version"`
	AuthURL     string            `json:"auth_url,omitempty,name=auth_url"`
	Container   string            `json:"swift_container,omitempty,name=swift_container"`
	StoragePath string            `json:"storage_path,omitempty,name=storage_path"`
	Username    string            `json:"swift_user,omitempty,name=swift_user"`
	Password    string            `json:"swift_password,omitempty,name=swift_password"`
	CACertPath  string            `json:"ca_cert_path,omitempty,name=ca_cert_path"`
	TempURLKey  string            `json:"temp_url_key,omitempty,name=temp_url_key"`
	OSOptions   map[string]string `json:"os_options,omitempty" protobuf:"bytes,7,rep,name=os_options"`
}

// CloudfrontS3RegistryBackendSource defines CouldfrontS3 registry storage
type CloudfrontS3RegistryBackendSource struct {
	StoragePath        string `json:"storage_path,omitempty,name=storage_path"`
	BucketName         string `json:"s3_bucket,omitempty,name=s3_bucket"`
	AccessKey          string `json:"s3_access_key,omitempty,name=s3_access_key"`
	SecretKey          string `json:"s3_secret_key,omitempty,name=s3_secret_key"`
	Host               string `json:"host,omitempty,name=host"`
	Port               int    `json:"port,omitempty,name=port"`
	KeyID              string `json:"cloudfront_distribution_domain,omitempty,name=cloudfront_distribution_domain"`
	DistributionDomain string `json:"cloudfront_key_id,omitempty,name=cloudfront_key_id"`
	PrivateKeyFilename string `json:"cloudfront_privatekey_filename,omitempty,name=cloudfront_privatekey_filename"`
}

// ExtraCaCert defines extra certificates that should be mounted
type ExtraCaCert struct {
	SecretName string   `json:"secretName"`
	Keys       []string `json:"keys,omitempty,name=keys"`
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
