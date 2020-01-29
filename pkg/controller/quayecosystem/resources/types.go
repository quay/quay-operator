package resources

import (
	"time"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"
)

// QuayConfiguration is a wrapper object around a QuayEcosystem that provides the full set of configurable options
type QuayConfiguration struct {
	QuayEcosystem *redhatcopv1alpha1.QuayEcosystem

	// Superuser
	QuaySuperuserUsername            string
	QuaySuperuserPassword            string
	QuaySuperuserEmail               string
	ValidProvidedQuaySuperuserSecret bool

	// Database
	ValidProvidedQuayDatabaseSecret bool
	QuayDatabase                    DatabaseConfig
	ProvisionQuayDatabase           bool

	// Database
	ValidProvidedClairDatabaseSecret bool
	ClairDatabase                    DatabaseConfig
	ProvisionClairDatabase           bool

	// Redis
	RedisHostname                    string
	RedisPort                        *int32
	RedisPassword                    string
	ValidProvidedRedisPasswordSecret bool

	// Quay
	QuayHostname                          string
	QuayConfigHostname                    string
	QuayConfigUsername                    string
	QuayConfigPassword                    string
	QuayConfigPasswordSecret              string
	ValidProvidedQuayConfigPasswordSecret bool
	QuayImage                             string
	QuayReplicas                          *int32
	DeployQuayConfiguration               bool
	QuaySslCertificate                    []byte
	QuaySslPrivateKey                     []byte
	SecurityScannerKeyID                  string
	RegistryBackends                      []redhatcopv1alpha1.RegistryBackend
	ConfigFiles                           []redhatcopv1alpha1.QuayConfigFiles

	// Clair
	ClairSslCertificate []byte
	ClairSslPrivateKey  []byte
	ClairUpdateInterval time.Duration

	IsOpenShift bool
}

// DatabaseConfig is an internal structure representing a database
type DatabaseConfig struct {
	Name                string
	Server              string
	Image               string
	Database            string
	Username            string
	Password            string
	RootPassword        string
	CPU                 string
	Memory              string
	VolumeSize          string
	CredentialsName     string
	ValidProvidedSecret bool
	UserProvided        bool
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

// SwiftRegistryBackendSource defines Swift registry storage
type SwiftRegistryBackendSource struct {
	AuthVersion string            `json:"auth_version,omitempty,name=auth_version"`
	AuthURL     string            `json:"auth_url,omitempty,name=auth_url"`
	Container   string            `json:"swift_container,omitempty,name=swift_container"`
	StoragePath string            `json:"storage_path,omitempty,name=storage_path"`
	User        string            `json:"swift_user,omitempty,name=swift_user"`
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
	DistributionDomain string `json:"cloudfront_distribution_domain,omitempty,name=cloudfront_distribution_domain"`
	KeyID              string `json:"cloudfront_key_id,omitempty,name=cloudfront_key_id"`
	PrivateKeyFilename string `json:"cloudfront_privatekey_filename,omitempty,name=cloudfront_privatekey_filename"`
}

// LocalRegistryBackendToQuayLocalRegistryBackend converts a LocalRegistryBackend Kubernetes resource to a Quay resource
func LocalRegistryBackendToQuayLocalRegistryBackend(localRegistryBackend *redhatcopv1alpha1.LocalRegistryBackendSource) LocalRegistryBackendSource {
	return LocalRegistryBackendSource{
		StoragePath: localRegistryBackend.StoragePath,
	}
}

// S3RegistryBackendToQuayS3RegistryBackend converts a S3RegistryBackend Kubernetes resource to a Quay resource
func S3RegistryBackendToQuayS3RegistryBackend(s3RegistryBackend *redhatcopv1alpha1.S3RegistryBackendSource) S3RegistryBackendSource {
	return S3RegistryBackendSource{
		AccessKey:   s3RegistryBackend.AccessKey,
		BucketName:  s3RegistryBackend.BucketName,
		Host:        s3RegistryBackend.Host,
		Port:        s3RegistryBackend.Port,
		SecretKey:   s3RegistryBackend.SecretKey,
		StoragePath: s3RegistryBackend.StoragePath,
	}
}

// GoogleCloudRegistryBackendToQuayGoogleCloudRegistryBackend converts a GoogleCloudRegistryBackendToQuayGoogleCloudRegistryBackend Kubernetes resource to a Quay resource
func GoogleCloudRegistryBackendToQuayGoogleCloudRegistryBackend(googleCloudRegistryBackend *redhatcopv1alpha1.GoogleCloudRegistryBackendSource) GoogleCloudRegistryBackendSource {
	return GoogleCloudRegistryBackendSource{
		AccessKey:   googleCloudRegistryBackend.AccessKey,
		BucketName:  googleCloudRegistryBackend.BucketName,
		SecretKey:   googleCloudRegistryBackend.SecretKey,
		StoragePath: googleCloudRegistryBackend.StoragePath,
	}
}

// AzureRegistryBackendToQuayAzureRegistryBackend converts a AzureRegistryBackendToQuayAzureRegistryBackend Kubernetes resource to a Quay resource
func AzureRegistryBackendToQuayAzureRegistryBackend(azureRegistryBackend *redhatcopv1alpha1.AzureRegistryBackendSource) AzureRegistryBackendSource {
	return AzureRegistryBackendSource{
		AccountKey:    azureRegistryBackend.AccountKey,
		AccountName:   azureRegistryBackend.AccountName,
		ContainerName: azureRegistryBackend.ContainerName,
		SasToken:      azureRegistryBackend.SasToken,
		StoragePath:   azureRegistryBackend.StoragePath,
	}
}

// RADOSRegistryBackendToQuayRADOSRegistryBackend converts a RADOSRegistryBackendToQuayRADOSRegistryBackend Kubernetes resource to a Quay resource
func RADOSRegistryBackendToQuayRADOSRegistryBackend(radosRegistryBackend *redhatcopv1alpha1.RADOSRegistryBackendSource) RADOSRegistryBackendSource {
	return RADOSRegistryBackendSource{
		AccessKey:   radosRegistryBackend.AccessKey,
		BucketName:  radosRegistryBackend.BucketName,
		Hostname:    radosRegistryBackend.Hostname,
		Port:        radosRegistryBackend.Port,
		SecretKey:   radosRegistryBackend.SecretKey,
		Secure:      radosRegistryBackend.Secure,
		StoragePath: radosRegistryBackend.StoragePath,
	}
}

// RHOCSRegistryBackendToQuayRHOCSRegistryBackend converts a RHOCSRegistryBackendToQuayRHOCSRegistryBackend Kubernetes resource to a Quay resource
func RHOCSRegistryBackendToQuayRHOCSRegistryBackend(rhocsRegistryBackend *redhatcopv1alpha1.RHOCSRegistryBackendSource) RHOCSRegistryBackendSource {
	return RHOCSRegistryBackendSource{
		AccessKey:   rhocsRegistryBackend.AccessKey,
		BucketName:  rhocsRegistryBackend.BucketName,
		Hostname:    rhocsRegistryBackend.Hostname,
		Port:        rhocsRegistryBackend.Port,
		SecretKey:   rhocsRegistryBackend.SecretKey,
		Secure:      rhocsRegistryBackend.Secure,
		StoragePath: rhocsRegistryBackend.StoragePath,
	}
}

// SwiftRegistryBackendToQuaySwiftRegistryBackend converts a SwiftRegistryBackendToQuaySwiftRegistryBackend Kubernetes resource to a Quay resource
func SwiftRegistryBackendToQuaySwiftRegistryBackend(swiftRegistryBackend *redhatcopv1alpha1.SwiftRegistryBackendSource) SwiftRegistryBackendSource {
	return SwiftRegistryBackendSource{
		AuthURL:     swiftRegistryBackend.AuthURL,
		AuthVersion: swiftRegistryBackend.AuthVersion,
		CACertPath:  swiftRegistryBackend.CACertPath,
		Container:   swiftRegistryBackend.Container,
		OSOptions:   swiftRegistryBackend.OSOptions,
		Password:    swiftRegistryBackend.Password,
		StoragePath: swiftRegistryBackend.StoragePath,
		TempURLKey:  swiftRegistryBackend.TempURLKey,
		User:        swiftRegistryBackend.User,
	}
}

// CloudfrontS3RegistryBackendToQuayCloudfrontS3RegistryBackend converts a CloudfrontS3RegistryBackendToQuayCloudfrontS3RegistryBackend Kubernetes resource to a Quay resource
func CloudfrontS3RegistryBackendToQuayCloudfrontS3RegistryBackend(cloudfrontS3RegistryBackend *redhatcopv1alpha1.CloudfrontS3RegistryBackendSource) CloudfrontS3RegistryBackendSource {
	return CloudfrontS3RegistryBackendSource{
		AccessKey:          cloudfrontS3RegistryBackend.AccessKey,
		BucketName:         cloudfrontS3RegistryBackend.BucketName,
		DistributionDomain: cloudfrontS3RegistryBackend.DistributionDomain,
		Host:               cloudfrontS3RegistryBackend.Host,
		KeyID:              cloudfrontS3RegistryBackend.KeyID,
		Port:               cloudfrontS3RegistryBackend.Port,
		PrivateKeyFilename: cloudfrontS3RegistryBackend.PrivateKeyFilename,
		SecretKey:          cloudfrontS3RegistryBackend.SecretKey,
		StoragePath:        cloudfrontS3RegistryBackend.StoragePath,
	}
}
