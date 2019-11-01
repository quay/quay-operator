package client

import (
	"net/url"
	"os"
	"time"
)

type RegistryStatus struct {
	Status string `json:"status"`
}

type QuayConfig struct {
	Config map[string]interface{} `json:"config"`
}

type QuayStatusResponse struct {
	Status bool   `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SetupDatabaseResponse struct {
	Logs []LogMessage `json:"logs"`
}

type LogMessage struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

type QuayCreateSuperuserRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmpassword"`
	Email           string `json:"email"`
}

type ConfigFileStatus struct {
	Exists bool `json:"exists"`
}

type KeyCreationRequest struct {
	Name       string     `json:"name"`
	Service    string     `json:"service"`
	Expiration *time.Time `json:"expiration"`
	Notes      string     `json:"notes"`
}

type KeyCreationResponse struct {
	KID        string `json:"kid"`
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	Service    string `json:"service"`
}

type StringValue struct {
	Value string
}

type Key struct {
	Approval         KeyApproval `json:"approval"`
	CreatedDate      string      `json:"created_date"`
	ExpirationDate   string      `json:"expiration_date,omitempty"`
	KID              string      `json:"kid"`
	Metadata         KeyMetadata `json:"metadata"`
	Jwk              KeyJwk      `json:"jwk"`
	Name             string      `json:"name"`
	RotationDuration string      `json:"rotationDuration,omitempty"`
	Service          string      `json:"service"`
}

type KeyApproval struct {
	ApprovalType string `json:"approval_type"`
	ApprovedDate string `json:"approved_date"`
	Approver     string `json:"approver,omitempty"`
	Notes        string `json:"notes"`
}

type KeyMetadata struct {
	CreatedBy string `json:"created_by"`
	IP        string `json:"ip,omitempty"`
}

type KeyJwk struct {
	E   string `json:"e"`
	Kty string `json:"kty"`
	N   string `json:"n"`
}

type KeysResponse struct {
	Keys []Key `json:"keys"`
}

type ClairFile struct {
	Clair    *ClairConfig   `yaml:"clair"`
	JwtProxy *ClairJwtProxy `yaml:"jwtproxy"`
}

type ClairConfig struct {
	Database *ClairDatabase `yaml:"database"`
	Updater  *ClairUpdater  `yaml:"updater"`
	Notifier *ClairNotifier `yaml:"notifier"`
	API      *ClairAPI      `yaml:"api"`
}

type ClairDatabase struct {
	Type    string                 `yaml:"type"`
	Options map[string]interface{} `yaml:"options"`
}

type ClairUpdater struct {
	Interval time.Duration `yaml:"interval"`
}

type ClairNotifier struct {
	Attempts         int                    `yaml:"attempts"`
	RenotifyInterval time.Duration          `yaml:"renotifyinterval"`
	Params           map[string]interface{} `yaml:",inline"`
}

type ClairAPI struct {
	Port          int           `yaml:"port"`
	HealthPort    int           `yaml:"healthport"`
	Timeout       time.Duration `yaml:"duration"`
	PaginationKey string        `yaml:"pagnationkey,omitempty"`
	CertFile      string        `yaml:"cert_file,omitempty"`
	KeyFile       string        `yaml:"key_file,omitempty"`
	CAFile        string        `yaml:"ca_file,omitempty"`
}

type ClairHttpNotifier struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	Proxy    string `yaml:"proxy,omitempty"`
}

type ClairJwtProxy struct {
	SignerProxy     SignerProxyConfig     `yaml:"signer_proxy"`
	VerifierProxies []VerifierProxyConfig `yaml:"verifier_proxies"`
}

type VerifierProxyConfig struct {
	Enabled          bool           `yaml:"enabled,omitempty"`
	ListenAddr       string         `yaml:"listen_addr,omitempty"`
	SocketPermission os.FileMode    `yaml:"socket_permission,omitempty"`
	ShutdownTimeout  time.Duration  `yaml:"shutdown_timeout,omitempty"`
	CrtFile          string         `yaml:"crt_file,omitempty"`
	KeyFile          string         `yaml:"key_file,omitempty"`
	Verifier         VerifierConfig `yaml:"verifier,omitempty"`
}

type SignerProxyConfig struct {
	Enabled             bool          `yaml:"enabled,omitempty"`
	ListenAddr          string        `yaml:"listen_addr,omitempty"`
	SocketPermission    os.FileMode   `yaml:"socket_permission,omitempty"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout,omitempty"`
	CAKeyFile           string        `yaml:"ca_key_file,omitempty"`
	CACrtFile           string        `yaml:"ca_crt_file,omitempty"`
	TrustedCertificates []string      `yaml:"trusted_certificates,omitempty"`
	InsecureSkipVerify  bool          `yaml:"insecure_skip_verify,omitempty"`
	Signer              SignerConfig  `yaml:"signer,omitempty"`
}

type VerifierConfig struct {
	Upstream        URL                          `yaml:"upstream,omitempty"`
	Audience        URL                          `yaml:"audience,omitempty"`
	MaxSkew         time.Duration                `yaml:"max_skew,omitempty"`
	MaxTTL          time.Duration                `yaml:"max_ttl,omitempty"`
	KeyServer       RegistrableComponentConfig   `yaml:"key_server,omitempty"`
	NonceStorage    RegistrableComponentConfig   `yaml:"nonce_storage,omitempty"`
	ClaimsVerifiers []RegistrableComponentConfig `yaml:"claims_verifiers,omitempty"`
}

type SignerParams struct {
	Issuer         string        `yaml:"issuer,omitempty"`
	ExpirationTime time.Duration `yaml:"expiration_time,omitempty"`
	MaxSkew        time.Duration `yaml:"max_skew,omitempty"`
	NonceLength    int           `yaml:"nonce_length,omitempty"`
}

type SignerConfig struct {
	SignerParams `yaml:",inline"`
	PrivateKey   RegistrableComponentConfig `yaml:"private_key,omitempty"`
}

type RegistrableComponentConfig struct {
	Type    string                 `yaml:"type"`
	Options map[string]interface{} `yaml:"options"`
}

type URL struct {
	URL *url.URL
}

// MarshalYAML implements the yaml.Marshaler interface for URLs.
func (u URL) MarshalYAML() (interface{}, error) {
	if u.URL != nil {
		return u.URL.String(), nil
	}
	return nil, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for URLs.
func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	urlp, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = urlp
	return nil
}
