package context

// QuayRegistryContext contains additional information accrued and consumed during the reconcile loop of a `QuayRegistry`.
type QuayRegistryContext struct {
	// External Access
	SupportsRoutes       bool
	ClusterHostname      string
	ServerHostname       string
	BuildManagerHostname string

	// Cluster CA Resource Versions
	ClusterServiceCAHash string
	ClusterTrustedCAHash string

	// TLS
	ClusterWildcardCert []byte
	TLSCert             []byte
	TLSKey              []byte

	// Object Storage
	SupportsObjectStorage    bool
	ObjectStorageInitialized bool
	StorageHostname          string
	StorageBucketName        string
	StorageAccessKey         string
	StorageSecretKey         string

	// Monitoring
	SupportsMonitoring bool

	// Secret Keys
	DatabaseSecretKey string
	SecretKey         string

	// Database
	DbUri               string
	DbRootPw            string
	NeedsPgUpgrade      bool
	NeedsClairPgUpgrade bool

	// Clair integration
	SecurityScannerV4PSK string
}

// NewQuayRegistryContext returns a fresh context for reconciling a `QuayRegistry`.
func NewQuayRegistryContext() *QuayRegistryContext {
	return &QuayRegistryContext{}
}
