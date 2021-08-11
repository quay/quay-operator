package context

// QuayRegistryContext contains additional information accrued and consumed during the reconcile loop of a `QuayRegistry`.
type QuayRegistryContext struct {
	// External Access
	SupportsRoutes       bool
	ClusterHostname      string
	ServerHostname       string
	BuildManagerHostname string

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
	DBURI string

	// Stores Clair PSK so we can reuse
	ClairSecurityScannerV4PSK string

	// Stores ConfigEditorPassword for reuse
	ConfigEditorPassword string
}

// NewQuayRegistryContext returns a fresh context for reconciling a `QuayRegistry`.
func NewQuayRegistryContext() *QuayRegistryContext {
	return &QuayRegistryContext{}
}
