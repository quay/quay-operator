package v1alpha1

import (
	"time"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"
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

// QuayConfigFileType defines the type of configuration file
type QuayConfigFileType string

// ExternalAccessType defines the method for accessing Quay from an external source
type ExternalAccessType string

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

	// ExtraCaCertQuayConfigFileType specifies a Extra Ca Certificate file type
	ExtraCaCertQuayConfigFileType QuayConfigFileType = "extraCaCert"

	// ConfigQuayConfigFileType specifies a Extra Ca Certificate file type
	ConfigQuayConfigFileType QuayConfigFileType = "config"

	// RouteExternalAccessType specifies external access using a Route
	RouteExternalAccessType ExternalAccessType = "Route"

	// LoadBalancerExternalAccessType specifies external access using a LoadBalancer
	LoadBalancerExternalAccessType ExternalAccessType = "LoadBalancer"

	// IngressExternalAccessType specifies external access using a Ingress
	IngressExternalAccessType ExternalAccessType = "Ingress"

	// NodePortExternalAccessType specifies external access using a NodePort
	NodePortExternalAccessType ExternalAccessType = "NodePort"
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
	// +listType=atomic
	Conditions    []QuayEcosystemCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
	SetupComplete bool                     `json:"setupComplete,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayEcosystem is the Schema for the quayecosystems API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=quayecosystems,scope=Namespaced
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
// +k8s:openapi-gen=true
type Quay struct {
	// +listType=atomic
	ConfigEnvVars    []corev1.EnvVar             `json:"configEnvVars,omitempty"`
	ConfigResources  corev1.ResourceRequirements `json:"configResources,omitempty" protobuf:"bytes,2,opt,name=configResources"`
	ConfigHostname   string                      `json:"configHostname,omitempty"`
	ConfigSecretName string                      `json:"configSecretName,omitempty"`
	// +listType=atomic
	RepoMirrorEnvVars        []corev1.EnvVar             `json:"repoMirrorEnvVars,omitempty"`
	RepoMirrorResources      corev1.ResourceRequirements `json:"repoMirrorResources,omitempty" protobuf:"bytes,2,opt,name=configResources"`
	RepoMirrorServerHostname string                      `json:"repoMirrorServerHostname,omitempty"`
	RepoMirrorTLSVerify      bool                        `json:"repoMirrorTLSVerify,omitempty"`

	Database *Database `json:"database,omitempty"`
	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy  appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	EnableRepoMirroring bool                          `json:"enableRepoMirroring,omitempty"`
	// +listType=atomic
	EnvVars              []corev1.EnvVar   `json:"envVars,omitempty"`
	Image                string            `json:"image,omitempty"`
	ImagePullSecretName  string            `json:"imagePullSecretName,omitempty"`
	LivenessProbe        *corev1.Probe     `json:"livenessProbe,omitempty"`
	KeepConfigDeployment bool              `json:"keepConfigDeployment,omitempty"`
	NodeSelector         map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	NodePort             *int32            `json:"nodePort,omitempty"`
	ConfigNodePort       *int32            `json:"configNodePort,omitempty"`
	ReadinessProbe       *corev1.Probe     `json:"readinessProbe,omitempty"`
	// +listType=atomic
	RegistryBackends               []RegistryBackend           `json:"registryBackends,omitempty"`
	RegistryStorage                *RegistryStorage            `json:"registryStorage,omitempty"`
	Replicas                       *int32                      `json:"replicas,omitempty"`
	Resources                      corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	Hostname                       string                      `json:"hostname,omitempty"`
	SkipSetup                      bool                        `json:"skipSetup,omitempty"`
	SslCertificatesSecretName      string                      `json:"sslCertificatesSecretName,omitempty"`
	SuperuserCredentialsSecretName string                      `json:"superuserCredentialsSecretName,omitempty"`
	EnableStorageReplication       bool                        `json:"enableStorageReplication,omitempty"`
	// +optional
	// +patchMergeKey=secretName
	// +patchStrategy=merge
	// +listType=atomic
	ConfigFiles []QuayConfigFiles `json:"configFiles,omitempty" patchStrategy:"merge" patchMergeKey:"secretName" protobuf:"bytes,2,rep,name=configFiles"`
	// +kubebuilder:validation:Enum=new-installation;add-new-fields;backfill-then-read-only-new;remove-old-field
	MigrationPhase QuayMigrationPhase `json:"migrationPhase,omitempty" protobuf:"bytes,1,opt,name=migrationPhase,casttype=QuayMigrationPhase"`

	// +kubebuilder:validation:Enum=Route;LoadBalancer;NodePort
	ExternalAccessType ExternalAccessType `json:"externalAccessType,omitempty"`
}

// QuayEcosystemCondition defines a list of conditions that the object will transiton through
// +k8s:openapi-gen=true
type QuayEcosystemCondition struct {
	LastTransitionTime metav1.Time                `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	LastUpdateTime     metav1.Time                `json:"lastUpdateTime,omitempty" protobuf:"bytes,3,opt,name=lastUpdateTime"`
	Message            string                     `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
	Reason             string                     `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	Status             corev1.ConditionStatus     `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/kubernetes/pkg/api/v1.ConditionStatus"`
	Type               QuayEcosystemConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=QuayEcosystemConditionType"`
}

// Redis defines the properies of a deployment of Redis
// +k8s:openapi-gen=true
type Redis struct {
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	// +listType=atomic
	EnvVars             []corev1.EnvVar             `json:"envVars,omitempty"`
	Hostname            string                      `json:"hostname,omitempty"`
	Image               string                      `json:"image,omitempty"`
	ImagePullSecretName string                      `json:"imagePullSecretName,omitempty"`
	LivenessProbe       *corev1.Probe               `json:"livenessProbe,omitempty"`
	NodeSelector        map[string]string           `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	Port                *int32                      `json:"port,omitempty"`
	ReadinessProbe      *corev1.Probe               `json:"readinessProbe,omitempty"`
	Replicas            *int32                      `json:"replicas,omitempty"`
	Resources           corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

// Database defines a database that will be deployed to support a particular component
// +k8s:openapi-gen=true
type Database struct {
	CPU                   string `json:"cpu,omitempty"`
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`

	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	// +listType=atomic
	EnvVars             []corev1.EnvVar             `json:"envVars,omitempty"`
	Image               string                      `json:"image,omitempty"`
	ImagePullSecretName string                      `json:"imagePullSecretName,omitempty"`
	LivenessProbe       *corev1.Probe               `json:"livenessProbe,omitempty"`
	Memory              string                      `json:"memory,omitempty"`
	NodeSelector        map[string]string           `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	ReadinessProbe      *corev1.Probe               `json:"readinessProbe,omitempty"`
	Replicas            *int32                      `json:"replicas,omitempty"`
	Resources           corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	Server              string                      `json:"server,omitempty"`
	VolumeSize          string                      `json:"volumeSize,omitempty"`
}

// Clair defines the properties of a deployment of Clair
// +k8s:openapi-gen=true
type Clair struct {
	Database *Database `json:"database,omitempty"`

	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`

	Enabled bool `json:"enabled,omitempty"`
	// +listType=atomic
	EnvVars                   []corev1.EnvVar             `json:"envVars,omitempty"`
	Image                     string                      `json:"image,omitempty"`
	ImagePullSecretName       string                      `json:"imagePullSecretName,omitempty"`
	LivenessProbe             *corev1.Probe               `json:"livenessProbe,omitempty"`
	NodeSelector              map[string]string           `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	ReadinessProbe            *corev1.Probe               `json:"readinessProbe,omitempty"`
	Replicas                  *int32                      `json:"replicas,omitempty"`
	Resources                 corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	SslCertificatesSecretName string                      `json:"sslCertificatesSecretName,omitempty"`
	UpdateInterval            string                      `json:"updateInterval,omitempty"`
}

// RegistryBackend defines a particular backend supporting the Quay registry
// +k8s:openapi-gen=true
type RegistryBackend struct {
	Name                  string `json:"name"`
	RegistryBackendSource `json:",inline" protobuf:"bytes,2,opt,name=registryBackendSource"`
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
	ReplicateByDefault    *bool  `json:"replicateByDefault,omitempty"`
}

// RegistryBackendSource defines the specific configurations to support the Quay registry
// +k8s:openapi-gen=true
type RegistryBackendSource struct {
	Local        *LocalRegistryBackendSource        `json:"local,omitempty,name=local"`
	S3           *S3RegistryBackendSource           `json:"s3,omitempty,name=s3"`
	GoogleCloud  *GoogleCloudRegistryBackendSource  `json:"googleCloud,omitempty,name=googleCloud"`
	Azure        *AzureRegistryBackendSource        `json:"azure,omitempty,name=azure"`
	RADOS        *RADOSRegistryBackendSource        `json:"rados,omitempty,name=rados"`
	RHOCS        *RHOCSRegistryBackendSource        `json:"rhocs,omitempty,name=rhocs"`
	Swift        *SwiftRegistryBackendSource        `json:"swift,omitempty,name=swift"`
	CloudfrontS3 *CloudfrontS3RegistryBackendSource `json:"cloudfrontS3,omitempty,name=cloudfrontS3"`
}

// RegistryStorage defines the configurations to support persistent storage
// +k8s:openapi-gen=true
type RegistryStorage struct {
	// +listType=set
	PersistentVolumeAccessModes      []corev1.PersistentVolumeAccessMode `json:"persistentVolumeAccessModes,omitempty,name=persistentVolumeAccessModes"`
	PersistentVolumeSize             string                              `json:"persistentVolumeSize,omitempty,name=volumeSize"`
	PersistentVolumeStorageClassName string                              `json:"persistentVolumeStorageClassName,omitempty,name=storageClassName"`
}

// LocalRegistryBackendSource defines local registry storage
// +k8s:openapi-gen=true
type LocalRegistryBackendSource struct {
	StoragePath string `json:"storagePath,omitempty,name=storagePath"`
}

// S3RegistryBackendSource defines S3 registry storage
// +k8s:openapi-gen=true
type S3RegistryBackendSource struct {
	StoragePath string `json:"storagePath,omitempty,name=storagePath"`
	BucketName  string `json:"bucketName,omitempty,name=bucketName"`
	AccessKey   string `json:"accessKey,omitempty,name=accessKey"`
	SecretKey   string `json:"secretKey,omitempty,name=secretKey"`
	Host        string `json:"host,omitempty,name=host"`
	Port        int    `json:"port,omitempty,name=port"`
}

// GoogleCloudRegistryBackendSource defines Google Cloud registry storage
// +k8s:openapi-gen=true
type GoogleCloudRegistryBackendSource struct {
	StoragePath string `json:"storagePath,omitempty,name=storagePath"`
	BucketName  string `json:"bucketName,omitempty,name=bucketName"`
	AccessKey   string `json:"accessKey,omitempty,name=accessKey"`
	SecretKey   string `json:"secretKey,omitempty,name=secretKey"`
}

// AzureRegistryBackendSource defines Azure blob registry storage
// +k8s:openapi-gen=true
type AzureRegistryBackendSource struct {
	StoragePath   string `json:"storagePath,omitempty,name=storagePath"`
	ContainerName string `json:"containerName,omitempty,name=containerName"`
	AccountName   string `json:"accountName,omitempty,name=accountName"`
	AccountKey    string `json:"accountKey,omitempty,name=accountKey"`
	SasToken      string `json:"sasToken,omitempty,name=sasToken"`
}

// RADOSRegistryBackendSource defines Ceph RADOS registry storage
// +k8s:openapi-gen=true
type RADOSRegistryBackendSource struct {
	StoragePath string `json:"storagePath,omitempty,name=storagePath"`
	BucketName  string `json:"bucketName,omitempty,name=bucketName"`
	AccessKey   string `json:"accessKey,omitempty,name=accessKey"`
	SecretKey   string `json:"secretKey,omitempty,name=secretKey"`
	Hostname    string `json:"hostname,omitempty,name=hostname"`
	Secure      bool   `json:"secure,omitempty,name=secure"`
	Port        int    `json:"port,omitempty,name=port"`
}

// RHOCSRegistryBackendSource defines RHOCS registry storage
// +k8s:openapi-gen=true
type RHOCSRegistryBackendSource struct {
	StoragePath string `json:"storagePath,omitempty,name=storagePath"`
	BucketName  string `json:"bucketName,omitempty,name=bucketName"`
	AccessKey   string `json:"accessKey,omitempty,name=accessKey"`
	SecretKey   string `json:"secretKey,omitempty,name=secretKey"`
	Hostname    string `json:"hostname,omitempty,name=hostname"`
	Secure      bool   `json:"secure,omitempty,name=secure"`
	Port        int    `json:"port,omitempty,name=port"`
}

// SwiftRegistryBackendSource defines Swift registry storage
// +k8s:openapi-gen=true
type SwiftRegistryBackendSource struct {
	StoragePath string            `json:"storagePath,omitempty,name=storagePath"`
	AuthVersion string            `json:"authVersion,omitempty,name=authVersion"`
	AuthURL     string            `json:"authURL,omitempty,name=authURL"`
	Container   string            `json:"container,omitempty,name=container"`
	User        string            `json:"user,omitempty,name=user"`
	Password    string            `json:"password,omitempty,name=password"`
	CACertPath  string            `json:"caCertPath,omitempty,name=caCertPath"`
	TempURLKey  string            `json:"tempURLKey,omitempty,name=tempURLKey"`
	OSOptions   map[string]string `json:"osOptions,omitempty" protobuf:"bytes,7,rep,name=osOptions"`
}

// CloudfrontS3RegistryBackendSource defines CouldfrontS3 registry storage
// +k8s:openapi-gen=true
type CloudfrontS3RegistryBackendSource struct {
	StoragePath        string `json:"storagePath,omitempty,name=storagePath"`
	BucketName         string `json:"bucketName,omitempty,name=bucketName"`
	AccessKey          string `json:"accessKey,omitempty,name=accessKey"`
	SecretKey          string `json:"secretKey,omitempty,name=secretKey"`
	Host               string `json:"host,omitempty,name=host"`
	Port               int    `json:"port,omitempty,name=port"`
	DistributionDomain string `json:"distributionDomain,omitempty,name=distributionDomain"`
	KeyID              string `json:"keyID,omitempty,name=keyID"`
	PrivateKeyFilename string `json:"privateKeyFilename,omitempty,name=privateKeyFilename"`
}

// QuayConfigFiles defines configuration files that are injected into the Quay resources
// +k8s:openapi-gen=true
type QuayConfigFiles struct {
	SecretName string `json:"secretName"`
	// +listType=atomic
	Files []QuayConfigFile   `json:"files,omitempty,name=files"`
	Type  QuayConfigFileType `json:"type,omitempty,name=type"`
}

// QuayConfigFile defines configuration files that are injected into the Quay resources
// +k8s:openapi-gen=true
type QuayConfigFile struct {
	// +kubebuilder:validation:Enum=config;extraCaCert
	Type     QuayConfigFileType `json:"type,omitempty,name=type"`
	Key      string             `json:"key,name=key"`
	Filename string             `json:"filename,omitempty,name=filename"`
}

type QuayMigrationPhase string

var (
	NewInstallation         QuayMigrationPhase = "new-installation"
	AddNewFields            QuayMigrationPhase = "add-new-fields"
	BackfillThenReadOnlyNew QuayMigrationPhase = "backfill-then-read-only-new"
	RemoveOldField          QuayMigrationPhase = "remove-old-field"
)

func init() {
	SchemeBuilder.Register(&QuayEcosystem{}, &QuayEcosystemList{})
}

// GetKeys returns the keys found in Configuration Files
func (quayConfigFiles *QuayConfigFiles) GetKeys() []string {
	keys := []string{}

	if !utils.IsZeroOfUnderlyingType(quayConfigFiles.Files) {
		for _, file := range quayConfigFiles.Files {
			if !utils.IsZeroOfUnderlyingType(file) {
				keys = append(keys, file.Key)
			}
		}
	}
	return keys

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
