package kustomize

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/quay/clair/v4/config"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/distributedstorage"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/securityscanner"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/api/v1"
)

const secretKeyLength = 80

// secretKeySecretName is the name of the Secret in which generated secret keys are
// stored.
const secretKeySecretName = "quay-registry-managed-secret-keys"

// SecretKeySecretName returns the name of the Secret in which generated secret keys are
// stored.
func SecretKeySecretName(quay *v1.QuayRegistry) string {
	return quay.GetName() + "-" + secretKeySecretName
}

// generateKeyIfMissing checks if the given key is in the parsed config. If not, the secretKeysSecret
// is checked for the key. If not present, a new key is generated.
func generateKeyIfMissing(parsedConfig map[string]interface{}, secretKeysSecret *corev1.Secret, keyName string, quay *v1.QuayRegistry, log logr.Logger) (string, *corev1.Secret) {
	// Check for the user-given key in config.
	found, ok := parsedConfig[keyName]
	if ok {
		log.Info("Secret key found in provided config", "keyName", keyName)
		return found.(string), secretKeysSecret
	}

	// If not found in the given config, check the secret keys Secret.
	if secretKeysSecret == nil {
		log.Info("Creating a new secret key Secret")
		secretKeysSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretKeySecretName(quay),
				Namespace: quay.Namespace,
			},
			StringData: map[string]string{},
		}
	}

	foundSecretKey, ok := secretKeysSecret.Data[keyName]
	if ok {
		log.Info("Secret key found in managed secret", "keyName", keyName)
		return string(foundSecretKey), secretKeysSecret
	} else {
		log.Info("Generating secret key", "keyName", keyName)
		generatedSecretKey, err := generateRandomString(secretKeyLength)
		check(err)

		stringData := secretKeysSecret.StringData
		if stringData == nil {
			stringData = map[string]string{}
		}

		secretKeysSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretKeySecretName(quay),
				Namespace: quay.Namespace,
			},
			Data:       secretKeysSecret.Data,
			StringData: stringData,
		}

		secretKeysSecret.StringData[keyName] = generatedSecretKey
		return generatedSecretKey, secretKeysSecret
	}
}

// handleSecretKeys generates any secret keys not already present in the config bundle and adds them
// to the specialized secretKeysSecret.
func handleSecretKeys(parsedConfig map[string]interface{}, secretKeysSecret *corev1.Secret, quay *v1.QuayRegistry, log logr.Logger) (string, string, *corev1.Secret) {
	// Check for SECRET_KEY and DATABASE_SECRET_KEY. If not present, generate them
	// and place them into their own Secret.
	secretKey, secretKeysSecret := generateKeyIfMissing(parsedConfig, secretKeysSecret, "SECRET_KEY", quay, log)
	databaseSecretKey, secretKeysSecret := generateKeyIfMissing(parsedConfig, secretKeysSecret, "DATABASE_SECRET_KEY", quay, log)
	return secretKey, databaseSecretKey, secretKeysSecret
}

// ConfigFileFor generates and returns the correct config YAML file data for the given component.
func ConfigFileFor(component string, quay *v1.QuayRegistry) ([]byte, error) {
	switch component {
	case "clair":
		fieldGroup, err := securityscanner.NewSecurityScannerFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		fieldGroup.FeatureSecurityScanner = true
		fieldGroup.SecurityScannerV4Endpoint = "http://" + quay.GetName() + "-" + "clair"
		fieldGroup.SecurityScannerV4NamespaceWhitelist = []string{"admin"}

		return yaml.Marshal(fieldGroup)
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

		return yaml.Marshal(fieldGroup)
	case "postgres":
		fieldGroup, err := database.NewDatabaseFieldGroup(map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		user := "postgres"
		// FIXME(alecmerdler): Make this more secure...
		password := "postgres"
		host := strings.Join([]string{quay.GetName(), "quay-postgres"}, "-")
		name := "quay"
		fieldGroup.DbUri = fmt.Sprintf("postgresql://%s:%s@%s/%s", user, password, host, name)

		return yaml.Marshal(fieldGroup)
	case "localstorage":
		fieldGroup := distributedstorage.DistributedStorageFieldGroup{
			DistributedStoragePreference:       []string{"default"},
			DistributedStorageDefaultLocations: []string{"default"},
			DistributedStorageConfig: map[string]distributedstorage.DistributedStorage{
				"default": {
					Name: "LocalStorage",
					Args: distributedstorage.DistributedStorageArgs{
						StoragePath: "/datastorage/registry",
					},
				},
			},
		}

		return yaml.Marshal(fieldGroup)
	// FIXME(alecmerdler): Needs to be just "minio"
	case "storage":
		fieldGroup := &distributedstorage.DistributedStorageFieldGroup{
			DistributedStoragePreference:       []string{"default"},
			DistributedStorageDefaultLocations: []string{"default"},
			DistributedStorageConfig: map[string]distributedstorage.DistributedStorage{
				"default": {
					Name: "RadosGWStorage",
					Args: distributedstorage.DistributedStorageArgs{
						Hostname:    strings.Join([]string{quay.GetName(), "quay-datastore"}, "-"),
						IsSecure:    false,
						Port:        9000,
						StoragePath: "/datastorage/registry",
						// FIXME(alecmerdler): Make this more secure...
						AccessKey:  "minio",
						SecretKey:  "minio123",
						BucketName: "quay-datastore",
					},
				},
			},
		}

		return yaml.Marshal(fieldGroup)
	default:
		return nil, errors.New("unknown component: " + component)
	}
}

// componentConfigFilesFor returns specific config files for managed components of a Quay registry.
func componentConfigFilesFor(component string, quay *v1.QuayRegistry) (map[string][]byte, error) {
	switch component {
	case "clair":
		return map[string][]byte{"config.yaml": clairConfigFor(quay)}, nil
	default:
		return nil, nil
	}
}

// clairConfigFor returns a Clair v4 config with the correct values.
func clairConfigFor(quay *v1.QuayRegistry) []byte {
	host := strings.Join([]string{quay.GetName(), "clair-postgres"}, "-")
	dbname := "clair"
	user := "postgres"
	// FIXME(alecmerdler): Make this more secure...
	password := "postgres"

	config := config.Config{
		HTTPListenAddr: ":8080",
		LogLevel:       "debug",
		Indexer: config.Indexer{
			ConnString:           fmt.Sprintf("host=%s port=5432 dbname=%s user=%s password=%s sslmode=disable", host, dbname, user, password),
			ScanLockRetry:        10,
			LayerScanConcurrency: 5,
			Migrations:           true,
		},
		Matcher: config.Matcher{
			ConnString:  fmt.Sprintf("host=%s port=5432 dbname=%s user=%s password=%s sslmode=disable", host, dbname, user, password),
			MaxConnPool: 100,
			Migrations:  true,
			IndexerAddr: "clair-indexer",
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
