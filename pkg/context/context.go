package context

// QuayRegistryContext contains additional information accrued and consumed during the reconcile loop of a `QuayRegistry`.
type QuayRegistryContext struct {
	// External Access
	SupportsRoutes       bool
	ClusterHostname      string
	ServerHostname       string
	BuildManagerHostname string

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
}

// NewQuayRegistryContext returns a fresh context for reconciling a `QuayRegistry`.
func NewQuayRegistryContext() *QuayRegistryContext {
	return &QuayRegistryContext{}
}
