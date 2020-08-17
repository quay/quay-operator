package kustomize

import (
	"errors"
	"fmt"
	"strings"

	"github.com/quay/clair/v4/config"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/distributedstorage"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/securityscanner"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/api/v1"
)

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
