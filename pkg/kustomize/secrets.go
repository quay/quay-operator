package kustomize

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/quay/clair/v4/config"
	"github.com/quay/clair/v4/notifier/webhook"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/distributedstorage"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/hostsettings"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/repomirror"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/securityscanner"
	"github.com/quay/config-tool/pkg/lib/shared"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/cert"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

// underTest can be switched on/off to ensure deterministic random string generation for testing output.
var underTest = false

const (
	secretKeySecretName = "quay-registry-managed-secret-keys"
	secretKeyLength     = 80

	clairService = "clair-app"
	// FIXME: Ensure this includes the `QuayRegistry` name prefix when we add `builder` managed component.
	buildmanRoute = "quay-builder"
)

// FieldGroupFor generates and returns the correct config field group for the given component.
func FieldGroupFor(ctx *quaycontext.QuayRegistryContext, component string, quay *v1.QuayRegistry) (shared.FieldGroup, error) {
	switch component {
	case "clair":
		fieldGroup, err := securityscanner.NewSecurityScannerFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		preSharedKey, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}
		psk := base64.StdEncoding.EncodeToString([]byte(preSharedKey))

		fieldGroup.FeatureSecurityScanner = true
		fieldGroup.SecurityScannerV4Endpoint = "http://" + quay.GetName() + "-" + clairService + ":80"
		fieldGroup.SecurityScannerV4NamespaceWhitelist = []string{"admin"}
		fieldGroup.SecurityScannerNotifications = true
		fieldGroup.SecurityScannerV4PSK = psk

		return fieldGroup, nil
	case "redis":
		fieldGroup, err := redis.NewRedisFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		fieldGroup.BuildlogsRedis = &redis.BuildlogsRedisStruct{
			Host: strings.Join([]string{quay.GetName(), "quay-redis"}, "-"),
			Port: 6379,
		}
		fieldGroup.UserEventsRedis = &redis.UserEventsRedisStruct{
			Host: strings.Join([]string{quay.GetName(), "quay-redis"}, "-"),
			Port: 6379,
		}

		return fieldGroup, nil
	case "postgres":
		fieldGroup, err := database.NewDatabaseFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		user := quay.GetName() + "-quay-database"
		name := quay.GetName() + "-quay-database"
		host := strings.Join([]string{quay.GetName(), "quay-database"}, "-")
		port := "5432"
		password, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}

		fieldGroup.DbUri = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", user, password, host, port, name)

		return fieldGroup, nil
	case "objectstorage":
		fieldGroup := &distributedstorage.DistributedStorageFieldGroup{
			FeatureProxyStorage:                true,
			DistributedStoragePreference:       []string{"local_us"},
			DistributedStorageDefaultLocations: []string{"local_us"},
			DistributedStorageConfig: map[string]*distributedstorage.DistributedStorageDefinition{
				"local_us": {
					Name: "RHOCSStorage",
					Args: &shared.DistributedStorageArgs{
						Hostname:    ctx.StorageHostname,
						IsSecure:    true,
						Port:        443,
						BucketName:  ctx.StorageBucketName,
						AccessKey:   ctx.StorageAccessKey,
						SecretKey:   ctx.StorageSecretKey,
						StoragePath: "/datastorage/registry",
					},
				},
			},
		}

		return fieldGroup, nil
	case "route":
		fieldGroup := &hostsettings.HostSettingsFieldGroup{
			ExternalTlsTermination: false,
			PreferredUrlScheme:     "https",
			ServerHostname:         ctx.ServerHostname,
		}

		return fieldGroup, nil
	case "mirror":
		fieldGroup := &repomirror.RepoMirrorFieldGroup{
			FeatureRepoMirror:   true,
			RepoMirrorInterval:  30,
			RepoMirrorTlsVerify: true,
		}

		return fieldGroup, nil
	case "horizontalpodautoscaler":
		return nil, nil
	default:
		return nil, errors.New("unknown component: " + component)
	}
}

// BaseConfig returns a minimum config bundle with values that Quay doesn't have defaults for.
func BaseConfig() map[string]interface{} {
	return map[string]interface{}{
		"FEATURE_MAILING":                    false,
		"REGISTRY_TITLE":                     "Red Hat Quay",
		"REGISTRY_TITLE_SHORT":               "Red Hat Quay",
		"AUTHENTICATION_TYPE":                "Database",
		"ENTERPRISE_LOGO_URL":                "/static/img/RH_Logo_Quay_Black_UX-horizontal.svg",
		"DEFAULT_TAG_EXPIRATION":             "2w",
		"ALLOW_PULLS_WITHOUT_STRICT_LOGGING": false,
		"TAG_EXPIRATION_OPTIONS":             []string{"2w"},
		"TEAM_RESYNC_STALE_TIME":             "60m",
		"FEATURE_DIRECT_LOGIN":               true,
		"FEATURE_BUILD_SUPPORT":              false,
		"TESTING":                            false,
	}
}

// EnsureTLSFor checks if given TLS cert/key pair are valid for the Quay registry to use for secure communication with clients,
// and generates a TLS certificate/key pair if they are not provided.
func EnsureTLSFor(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, tlsCert, tlsKey []byte) ([]byte, []byte, error) {
	fieldGroup, err := FieldGroupFor(ctx, "route", quay)
	if err != nil {
		return tlsCert, tlsKey, err
	}

	routeFieldGroup := fieldGroup.(*hostsettings.HostSettingsFieldGroup)

	hosts := []string{
		routeFieldGroup.ServerHostname,
	}

	// Only add BUILDMAN_HOSTNAME as host if provided.
	if ctx.BuildManagerHostname != "" {
		hosts = append(hosts, strings.Split(ctx.BuildManagerHostname, ":")[0])
	}

	if tlsCert == nil && tlsKey == nil {
		return cert.GenerateSelfSignedCertKey(routeFieldGroup.ServerHostname, []net.IP{}, hosts)
	}

	for _, host := range hosts {
		if valid, validationErr := shared.ValidateCertPairWithHostname(tlsCert, tlsKey, host, fieldGroupNameFor("route")); !valid {
			return nil, nil, fmt.Errorf("provided certificate/key pair not valid for host '%s': %s", host, validationErr.String())
		}
	}

	return tlsCert, tlsKey, nil
}

// ContainsComponentConfig accepts a full `config.yaml` and determines if it contains
// the fieldgroup for the given component by comparing it with the fieldgroup defaults.
// TODO: Replace this with function from `config-tool` library once implemented.
func ContainsComponentConfig(fullConfig map[string]interface{}, component string) (bool, error) {
	fields := []string{}

	switch component {
	case "clair":
		fields = (&securityscanner.SecurityScannerFieldGroup{}).Fields()
	case "postgres":
		fields = (&database.DatabaseFieldGroup{}).Fields()
	case "redis":
		fields = (&redis.RedisFieldGroup{}).Fields()
	case "objectstorage":
		fields = (&distributedstorage.DistributedStorageFieldGroup{}).Fields()
	case "horizontalpodautoscaler":
		// HorizontalPodAutoscaler has no associated config fieldgroup.
		return false, nil
	case "mirror":
		fields = (&repomirror.RepoMirrorFieldGroup{}).Fields()
	case "route":
		for _, field := range (&hostsettings.HostSettingsFieldGroup{}).Fields() {
			// SERVER_HOSTNAME is a special field which we allow when using managed `route` component.
			if field != "SERVER_HOSTNAME" {
				fields = append(fields, field)
			}
		}
	default:
		panic("unknown component: " + component)
	}

	// FIXME: Only checking for the existance of a single field
	for _, field := range fields {
		if _, ok := fullConfig[field]; ok {
			return true, nil
		}
	}

	return false, nil
}

func fieldGroupNameFor(component string) string {
	switch component {
	case "clair":
		return "SecurityScanner"
	case "postgres":
		return "Database"
	case "redis":
		return "Redis"
	case "objectstorage":
		return "DistributedStorage"
	case "route":
		return "HostSettings"
	case "mirror":
		return "RepoMirror"
	case "horizontalpodautoscaler":
		return ""
	default:
		panic("unknown component: " + component)
	}
}

// componentConfigFilesFor returns specific config files for managed components of a Quay registry.
func componentConfigFilesFor(component string, quay *v1.QuayRegistry, configFiles map[string][]byte) (map[string][]byte, error) {
	switch component {
	case "postgres":
		dbConfig, ok := configFiles["postgres.config.yaml"]
		if !ok {
			return nil, fmt.Errorf("cannot generate managed component config file for `postgres` if `postgres.config.yaml` is missing")
		}

		var fieldGroup database.DatabaseFieldGroup
		if err := yaml.Unmarshal(dbConfig, &fieldGroup); err != nil {
			return nil, err
		}

		dbURI, err := url.Parse(fieldGroup.DbUri)
		if err != nil {
			return nil, err
		}

		databaseUsername := dbURI.User.Username()
		databasePassword, _ := dbURI.User.Password()
		databaseName := dbURI.Path[1:]
		databaseRootPassword, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}

		return map[string][]byte{
			"database-username":      []byte(databaseUsername),
			"database-password":      []byte(databasePassword),
			"database-name":          []byte(databaseName),
			"database-root-password": []byte(databaseRootPassword),
		}, nil
	case "clair":
		quayHostname := ""
		if v1.ComponentIsManaged(quay.Spec.Components, "route") {
			config := decode(configFiles["route.config.yaml"])
			quayHostname = config.(map[string]interface{})["SERVER_HOSTNAME"].(string)
		}

		if _, ok := configFiles["config.yaml"]; ok && quayHostname == "" {
			config := decode(configFiles["config.yaml"])
			if configHostname, ok := config.(map[string]interface{})["SERVER_HOSTNAME"].(string); ok && configHostname != "" {
				quayHostname = configHostname
			}
		}

		if quayHostname == "" {
			return nil, errors.New("cannot configure managed security scanner, `SERVER_HOSTNAME` is not set anywhere")
		}

		var preSharedKey string
		if _, ok := configFiles["clair.config.yaml"]; ok {
			config := decode(configFiles["clair.config.yaml"])
			preSharedKey = config.(map[string]interface{})["SECURITY_SCANNER_V4_PSK"].(string)
		}

		return map[string][]byte{"config.yaml": clairConfigFor(quay, quayHostname, preSharedKey)}, nil
	default:
		return nil, nil
	}
}

// clairConfigFor returns a Clair v4 config with the correct values.
func clairConfigFor(quay *v1.QuayRegistry, quayHostname, preSharedKey string) []byte {
	host := strings.Join([]string{quay.GetName(), "clair-postgres"}, "-")
	dbname := "postgres"
	user := "postgres"
	password := "postgres"

	psk, err := base64.StdEncoding.DecodeString(preSharedKey)
	check(err)

	dbConn := fmt.Sprintf("host=%s port=5432 dbname=%s user=%s password=%s sslmode=disable", host, dbname, user, password)
	config := config.Config{
		HTTPListenAddr: ":8080",
		LogLevel:       "info",
		Indexer: config.Indexer{
			ConnString:           dbConn,
			ScanLockRetry:        10,
			LayerScanConcurrency: 5,
			Migrations:           true,
		},
		Matcher: config.Matcher{
			ConnString:  dbConn,
			MaxConnPool: 100,
			Migrations:  true,
		},
		Notifier: config.Notifier{
			ConnString:       dbConn,
			Migrations:       true,
			DeliveryInterval: "1m",
			PollInterval:     "5m",
			Webhook: &webhook.Config{
				// NOTE: This can't be the in-cluster service hostname because the `passthrough` TLS certs are only valid for external `SERVER_HOSTNAME`.
				Target:   "https://" + quayHostname + "/secscan/notification",
				Callback: "http://" + quay.GetName() + "-" + clairService + "/notifier/api/v1/notifications",
			},
		},
		Auth: config.Auth{
			PSK: &config.AuthPSK{
				Key:    psk,
				Issuer: []string{"quay", "clairctl"},
			},
		},
		Metrics: config.Metrics{
			Name: "prometheus",
		},
	}

	marshalled, err := yaml.Marshal(config)
	check(err)

	return marshalled
}

// From: https://gist.github.com/dopey/c69559607800d2f2f90b1b1ed4e550fb
// generateRandomBytes returns securely generated random bytes.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// generateRandomString returns a securely generated random string.
func generateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	bytes, err := generateRandomBytes(n)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes), nil
}
