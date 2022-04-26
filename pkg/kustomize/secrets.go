package kustomize

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/quay/clair/config"
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

const (
	secretKeyLength = 80
)

// FieldGroupFor generates and returns the correct config field group for the given component.
func FieldGroupFor(
	ctx *quaycontext.QuayRegistryContext, component v1.ComponentKind, quay *v1.QuayRegistry,
) (shared.FieldGroup, error) {
	switch component {
	case v1.ComponentClair:
		config := map[string]interface{}{}
		fieldGroup, err := securityscanner.NewSecurityScannerFieldGroup(config)
		if err != nil {
			return nil, err
		}

		if len(ctx.SecurityScannerV4PSK) == 0 {
			preSharedKey, err := generateRandomString(32)
			if err != nil {
				return nil, err
			}

			ctx.SecurityScannerV4PSK = base64.StdEncoding.EncodeToString(
				[]byte(preSharedKey),
			)
		}

		fieldGroup.FeatureSecurityScanner = true
		fieldGroup.SecurityScannerV4Endpoint = fmt.Sprintf(
			"http://%s-clair-app.%s.svc.cluster.local",
			quay.GetName(),
			quay.GetNamespace(),
		)
		fieldGroup.SecurityScannerV4NamespaceWhitelist = []string{"admin"}
		fieldGroup.SecurityScannerNotifications = true
		fieldGroup.SecurityScannerV4PSK = ctx.SecurityScannerV4PSK
		return fieldGroup, nil

	case v1.ComponentRedis:
		fieldGroup, err := redis.NewRedisFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		fieldGroup.BuildlogsRedis = &redis.BuildlogsRedisStruct{
			Host: fmt.Sprintf("%s-quay-redis", quay.GetName()),
			Port: 6379,
		}
		fieldGroup.UserEventsRedis = &redis.UserEventsRedisStruct{
			Host: fmt.Sprintf("%s-quay-redis", quay.GetName()),
			Port: 6379,
		}
		return fieldGroup, nil

	case v1.ComponentPostgres:
		fieldGroup, err := database.NewDatabaseFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		fieldGroup.DbUri = ctx.DbUri

		// XXX after bumping database package (dependency) these fields stopped being
		// set to true by default. These lines restores the old behavior so we don't
		// expect to have unexpected side effects.
		if fieldGroup.DbConnectionArgs != nil {
			fieldGroup.DbConnectionArgs.Autorollback = true
			fieldGroup.DbConnectionArgs.Threadlocals = true
		}

		return fieldGroup, nil

	case v1.ComponentObjectStorage:
		storageConfig := map[string]*distributedstorage.DistributedStorageDefinition{
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
		}

		return &distributedstorage.DistributedStorageFieldGroup{
			FeatureProxyStorage:                true,
			DistributedStoragePreference:       []string{"local_us"},
			DistributedStorageDefaultLocations: []string{"local_us"},
			DistributedStorageConfig:           storageConfig,
		}, nil

	case v1.ComponentRoute:
		// sets tls termination in the load balancer if no cert has been provided.
		terminateExternally := len(ctx.TLSCert) == 0 && len(ctx.TLSKey) == 0
		fieldGroup := &hostsettings.HostSettingsFieldGroup{
			ExternalTlsTermination: terminateExternally,
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

	case v1.ComponentClairPostgres:
		return nil, nil

	case v1.ComponentQuay:
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown component: %s", component)
	}
}

// BaseQuayConfig returns a minimum config bundle with values that Quay doesn't have defaults for.
func BaseQuayConfig() map[string]interface{} {
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
		"FEATURE_QUOTA_MANAGEMENT":           true,
	}
}

// EnsureTLSFor checks if given TLS cert/key pair are valid for the Quay registry to use for
// secure communication with clients.
func EnsureTLSFor(
	ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) ([]byte, []byte, error) {
	fieldGroup, err := FieldGroupFor(ctx, v1.ComponentRoute, quay)
	if err != nil {
		return nil, nil, err
	}

	routeFieldGroup, ok := fieldGroup.(*hostsettings.HostSettingsFieldGroup)
	if !ok {
		return nil, nil, fmt.Errorf("invalid field group found")
	}

	hosts := []string{routeFieldGroup.ServerHostname}
	if ctx.BuildManagerHostname != "" {
		hosts = append(hosts, strings.Split(ctx.BuildManagerHostname, ":")[0])
	}

	fgn, err := v1.FieldGroupNameFor(v1.ComponentRoute)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting group name for component: %w", err)
	}

	if ctx.TLSCert != nil && ctx.TLSKey != nil {
		for _, host := range hosts {
			if valid, err := shared.ValidateCertPairWithHostname(
				ctx.TLSCert, ctx.TLSKey, host, fgn,
			); !valid {
				return nil, nil, fmt.Errorf(
					"provided certificate pair not valid for host '%s': %s",
					host,
					err,
				)
			}
		}
	}

	return ctx.TLSCert, ctx.TLSKey, nil
}

// ContainsComponentConfig accepts a full configBundleSecret and determines if it contains
// the fieldgroup for the given component by comparing it with the fieldgroup defaults.
// TODO: Replace this with function from `config-tool` library once implemented.
func ContainsComponentConfig(
	configBundle map[string][]byte, component v1.Component,
) (bool, error) {
	fields := []string{}

	switch component.Kind {
	case v1.ComponentClair:
		fields = (&securityscanner.SecurityScannerFieldGroup{}).Fields()

	case v1.ComponentPostgres:
		fields = (&database.DatabaseFieldGroup{}).Fields()

	case v1.ComponentClairPostgres:
		clairConfigBytes, ok := configBundle["clair-config.yaml"]
		// Clair config not provided
		if !ok {
			return false, nil
		}
		var clairConfig config.Config
		if err := yaml.Unmarshal(clairConfigBytes, &clairConfig); err != nil {
			return false, err
		}
		// Else check if connstring is provided anywhere in the config
		if clairConfig.Matcher.ConnString != "" ||
			clairConfig.Indexer.ConnString != "" ||
			clairConfig.Notifier.ConnString != "" {
			return true, nil
		}
		return false, nil

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
		_, keyPresent := configBundle["ssl.key"]
		_, certPresent := configBundle["ssl.cert"]
		if certPresent && keyPresent {
			return true, nil
		}

	case v1.ComponentQuay:
		return false, nil

	default:
		return false, fmt.Errorf("unknown component: %s", component.Kind)
	}

	var quayConfig map[string]interface{}
	err := yaml.Unmarshal(configBundle["config.yaml"], &quayConfig)
	if err != nil {
		return false, err
	}

	// FIXME: Only checking for the existance of a single field
	for _, field := range fields {
		if _, ok := quayConfig[field]; ok {
			return true, nil
		}
	}

	return false, nil
}

// componentConfigFilesFor returns specific config files for managed components of a Quay registry.
func componentConfigFilesFor(log logr.Logger, qctx *quaycontext.QuayRegistryContext, component v1.ComponentKind, quay *v1.QuayRegistry, configFiles map[string][]byte) (map[string][]byte, error) {
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
		databaseRootPassword := qctx.DbRootPw

		return map[string][]byte{
			"database-username":      []byte(databaseUsername),
			"database-password":      []byte(databasePassword),
			"database-name":          []byte(databaseName),
			"database-root-password": []byte(databaseRootPassword),
		}, nil
	case v1.ComponentClair:
		cfgFiles := make(map[string][]byte)

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

		// Add ssl key and cert to bundle
		if val, ok := configFiles["clair-ssl.crt"]; ok {
			cfgFiles["clair-ssl.crt"] = val
		}
		if val, ok := configFiles["clair-ssl.key"]; ok {
			cfgFiles["clair-ssl.key"] = val
		}

		if quayHostname == "" {
			return nil, fmt.Errorf("cannot configure managed security scanner, `SERVER_HOSTNAME` is not set anywhere")
		}

		var preSharedKey string
		if _, ok := configFiles["clair.config.yaml"]; ok {
			config := decode(configFiles["clair.config.yaml"])
			preSharedKey = config.(map[string]interface{})["SECURITY_SCANNER_V4_PSK"].(string)
		}

		cfg, err := clairConfigFor(log, quay, quayHostname, preSharedKey, configFiles)
		if err != nil {
			return nil, err
		}
		cfgFiles["config.yaml"] = cfg

		return cfgFiles, nil
	default:
		return nil, nil
	}
}

// clairConfigFor returns a Clair v4 config with the correct values.
func clairConfigFor(log logr.Logger, quay *v1.QuayRegistry, quayHostname, preSharedKey string, configFiles map[string][]byte) ([]byte, error) {

	host := strings.Join([]string{quay.GetName(), "clair-postgres"}, "-")
	dbname := "postgres"
	user := "postgres"
	password := "postgres"
	dbConn := fmt.Sprintf("host=%s port=5432 dbname=%s user=%s password=%s sslmode=disable", host, dbname, user, password)

	psk, err := base64.StdEncoding.DecodeString(preSharedKey)
	if err != nil {
		return nil, err
	}

	cfg := map[string]interface{}{
		"http_listen_addr": ":8080",
		"log_level":        "info",
		"indexer": map[string]interface{}{
			"connstring":             dbConn,
			"scanlock_retry":         10,
			"layer_scan_concurrency": 5,
			"migrations":             true,
		},
		"matcher": map[string]interface{}{
			"connstring":    dbConn,
			"max_conn_pool": 100,
			"migrations":    true,
		},
		"notifier": map[string]interface{}{
			"connstring":        dbConn,
			"migrations":        true,
			"delivery_interval": 1 * time.Minute,
			"poll_interval":     5 * time.Minute,
			"webhook": map[string]interface{}{
				"target":   "https://" + quayHostname + "/secscan/notification",
				"callback": "http://" + quay.GetName() + "-clair-app/notifier/api/v1/notifications",
			},
		},
		"auth": map[string]interface{}{
			"psk": map[string]interface{}{
				"key": config.Base64(psk),
				"iss": []string{"quay", "clairctl"},
			},
		},
		"metrics": map[string]interface{}{
			"name": "prometheus",
		},
	}

	// Overwrite default values with user provided clair configuration.
	if clairConfig, ok := configFiles["clair-config.yaml"]; ok {
		err := yaml.Unmarshal(clairConfig, &cfg)
		if err != nil {
			return nil, err
		}
	}

	return yaml.Marshal(cfg)
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
