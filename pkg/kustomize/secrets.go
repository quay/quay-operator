package kustomize

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
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

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

// underTest can be switched on/off to ensure deterministic random string generation for testing output.
var underTest = false

const (
	secretKeyLength = 80

	clairService = "clair-app"
	// FIXME: Ensure this includes the `QuayRegistry` name prefix when we add `builder` managed component.
	buildmanRoute = "quay-builder"
)

// FieldGroupFor generates and returns the correct config field group for the given component.
func FieldGroupFor(ctx *quaycontext.QuayRegistryContext, component v1.ComponentKind, quay *v1.QuayRegistry) (shared.FieldGroup, error) {
	switch component {
	case v1.ComponentClair:
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
	case v1.ComponentRedis:
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
	case v1.ComponentPostgres:
		fieldGroup, err := database.NewDatabaseFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		fieldGroup.DbUri = ctx.DbUri

		return fieldGroup, nil
	case v1.ComponentObjectStorage:
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
	case v1.ComponentRoute:
		fieldGroup := &hostsettings.HostSettingsFieldGroup{
			ExternalTlsTermination: true,
			PreferredUrlScheme:     "https",
			ServerHostname:         ctx.ServerHostname,
		}

		return fieldGroup, nil
	case v1.ComponentMirror:
		fieldGroup := &repomirror.RepoMirrorFieldGroup{
			FeatureRepoMirror:   true,
			RepoMirrorInterval:  30,
			RepoMirrorTlsVerify: true,
		}

		return fieldGroup, nil
	case v1.ComponentHPA:
		return nil, nil
	case v1.ComponentMonitoring:
		return nil, nil
	case v1.ComponentTLS:
		return nil, nil
	default:
		return nil, errors.New("unknown component: " + string(component))
	}
}

// BaseConfig returns a minimum config bundle with values that Quay doesn't have defaults for.
func BaseConfig() map[string]interface{} {
	registryTitle := "Quay"
	enterpriseLogoURL := "/static/img/quay-horizontal-color.svg"
	if os.Getenv("QUAY_DEFAULT_BRANDING") == "redhat" {
		registryTitle = "Red Hat Quay"
		enterpriseLogoURL = "/static/img/RH_Logo_Quay_Black_UX-horizontal.svg"
	}

	return map[string]interface{}{
		"SETUP_COMPLETE":                     true,
		"FEATURE_MAILING":                    false,
		"REGISTRY_TITLE":                     registryTitle,
		"REGISTRY_TITLE_SHORT":               registryTitle,
		"AUTHENTICATION_TYPE":                "Database",
		"ENTERPRISE_LOGO_URL":                enterpriseLogoURL,
		"DEFAULT_TAG_EXPIRATION":             "2w",
		"ALLOW_PULLS_WITHOUT_STRICT_LOGGING": false,
		"TAG_EXPIRATION_OPTIONS":             []string{"2w"},
		"TEAM_RESYNC_STALE_TIME":             "60m",
		"FEATURE_DIRECT_LOGIN":               true,
		"FEATURE_BUILD_SUPPORT":              false,
		"TESTING":                            false,
	}
}

// EnsureTLSFor checks if given TLS cert/key pair are valid for the Quay registry to use for secure communication with clients.
func EnsureTLSFor(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry) ([]byte, []byte, error) {
	fieldGroup, err := FieldGroupFor(ctx, "route", quay)
	if err != nil {
		return ctx.TLSCert, ctx.TLSKey, err
	}

	routeFieldGroup := fieldGroup.(*hostsettings.HostSettingsFieldGroup)

	hosts := []string{
		routeFieldGroup.ServerHostname,
	}

	// Only add BUILDMAN_HOSTNAME as host if provided.
	if ctx.BuildManagerHostname != "" {
		hosts = append(hosts, strings.Split(ctx.BuildManagerHostname, ":")[0])
	}

	if ctx.TLSCert != nil && ctx.TLSKey != nil {
		for _, host := range hosts {
			if valid, validationErr := shared.ValidateCertPairWithHostname(ctx.TLSCert, ctx.TLSKey, host, fieldGroupNameFor("route")); !valid {
				return nil, nil, fmt.Errorf("provided certificate/key pair not valid for host '%s': %s", host, validationErr.String())
			}
		}
	}

	return ctx.TLSCert, ctx.TLSKey, nil
}

// ContainsComponentConfig accepts a full `config.yaml` and determines if it contains
// the fieldgroup for the given component by comparing it with the fieldgroup defaults.
// TODO: Replace this with function from `config-tool` library once implemented.
func ContainsComponentConfig(fullConfig map[string]interface{}, certs map[string][]byte, component v1.Component) (bool, error) {
	fields := []string{}

	switch component.Kind {
	case v1.ComponentClair:
		fields = (&securityscanner.SecurityScannerFieldGroup{}).Fields()
	case v1.ComponentPostgres:
		fields = (&database.DatabaseFieldGroup{}).Fields()
	case v1.ComponentRedis:
		fields = (&redis.RedisFieldGroup{}).Fields()
	case v1.ComponentObjectStorage:
		fields = (&distributedstorage.DistributedStorageFieldGroup{}).Fields()
	case v1.ComponentHPA:
		// HorizontalPodAutoscaler has no associated config fieldgroup.
		return false, nil
	case v1.ComponentMirror:
		fields = (&repomirror.RepoMirrorFieldGroup{}).Fields()
	case v1.ComponentRoute:
		fields = (&hostsettings.HostSettingsFieldGroup{}).Fields()
	case v1.ComponentMonitoring:
		return false, nil
	case v1.ComponentTLS:
		_, keyPresent := certs["ssl.key"]
		_, certPresent := certs["ssl.cert"]
		if certPresent && keyPresent {
			return true, nil
		}
	default:
		panic("unknown component: " + component.Kind)
	}

	// FIXME: Only checking for the existance of a single field
	for _, field := range fields {
		if _, ok := fullConfig[field]; ok {
			return true, nil
		}
	}

	return false, nil
}

func fieldGroupNameFor(component v1.ComponentKind) string {
	switch component {
	case v1.ComponentClair:
		return "SecurityScanner"
	case v1.ComponentPostgres:
		return "Database"
	case v1.ComponentRedis:
		return "Redis"
	case v1.ComponentObjectStorage:
		return "DistributedStorage"
	case v1.ComponentRoute:
		return "HostSettings"
	case v1.ComponentMirror:
		return "RepoMirror"
	case v1.ComponentHPA:
		return ""
	case v1.ComponentMonitoring:
		return ""
	case v1.ComponentTLS:
		return ""
	default:
		panic("unknown component: " + component)
	}
}

// componentConfigFilesFor returns specific config files for managed components of a Quay registry.
func componentConfigFilesFor(component v1.ComponentKind, quay *v1.QuayRegistry, configFiles map[string][]byte) (map[string][]byte, error) {
	switch component {
	case v1.ComponentPostgres:
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
	case v1.ComponentClair:
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
