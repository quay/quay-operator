package database

import (
	"errors"

	"github.com/creasty/defaults"
)

// DatabaseFieldGroup represents the DatabaseFieldGroup config fields
type DatabaseFieldGroup struct {
	DbConnectionArgs *DbConnectionArgsStruct `default:"{}" json:"DB_CONNECTION_ARGS,omitempty" yaml:"DB_CONNECTION_ARGS,omitempty"`
	DbUri            string                  `default:"" validate:"" json:"DB_URI,omitempty" yaml:"DB_URI,omitempty"`
}

// DbConnectionArgsStruct represents the DbConnectionArgsStruct config fields
type DbConnectionArgsStruct struct {
	// MySQL arguments
	Ssl          *SslStruct `default:""  json:"ssl,omitempty" yaml:"ssl,omitempty"`
	Threadlocals bool       `default:""  json:"threadlocals,omitempty" yaml:"threadlocals,omitempty"`
	Autorollback bool       `default:""  json:"autorollback,omitempty" yaml:"autorollback,omitempty"`

	// Postgres arguments
	SslRootCert        string `default:""  json:"sslrootcert,omitempty" yaml:"sslrootcert,omitempty"`
	SslMode            string `default:""  json:"sslmode,omitempty" yaml:"sslmode,omitempty"`
	MaxConnections     int    `default:"10" json:"max_connections,omitempty" yaml:"max_connections,omitempty"`
	StaleTimeout       int    `default:"180" json:"stale_timeout,omitempty" yaml:"stale_timeout,omitempty"`
	KeepAlives         int    `default:"1" json:"keepalives,omitempty" yaml:"keepalives,omitempty"`
	KeepalivesIdle     int    `default:"10" json:"keepalives_idle,omitempty" yaml:"keepalives_idle,omitempty"`
	KeepalivesInterval int    `default:"2" json:"keepalives_interval,omitempty" yaml:"keepalives_interval,omitempty"`
	KeepalivesCount    int    `default:"5" json:"keepalives_count,omitempty" yaml:"keepalives_count,omitempty"`
}

// SslStruct represents the SslStruct config fields
type SslStruct struct {
	Ca string `default:"" validate:"" json:"ca,omitempty" yaml:"ca,omitempty"`
}

// NewDatabaseFieldGroup creates a new DatabaseFieldGroup
func NewDatabaseFieldGroup(fullConfig map[string]interface{}) (*DatabaseFieldGroup, error) {
	newDatabaseFieldGroup := &DatabaseFieldGroup{}
	defaults.Set(newDatabaseFieldGroup)

	if value, ok := fullConfig["DB_CONNECTION_ARGS"]; ok {
		var err error
		value := value.(map[string]interface{})
		newDatabaseFieldGroup.DbConnectionArgs, err = NewDbConnectionArgsStruct(value)
		if err != nil {
			return newDatabaseFieldGroup, err
		}
	}
	if value, ok := fullConfig["DB_URI"]; ok {
		newDatabaseFieldGroup.DbUri, ok = value.(string)
		if !ok {
			return newDatabaseFieldGroup, errors.New("DB_URI must be of type string")
		}
	}

	return newDatabaseFieldGroup, nil
}

// NewDbConnectionArgsStruct creates a new DbConnectionArgsStruct
func NewDbConnectionArgsStruct(fullConfig map[string]interface{}) (*DbConnectionArgsStruct, error) {
	newDbConnectionArgsStruct := &DbConnectionArgsStruct{}
	defaults.Set(newDbConnectionArgsStruct)

	if value, ok := fullConfig["ssl"]; ok {
		var err error
		value := value.(map[string]interface{})
		newDbConnectionArgsStruct.Ssl, err = NewSslStruct(value)
		if err != nil {
			return newDbConnectionArgsStruct, err
		}
	}
	if value, ok := fullConfig["threadlocals"]; ok {
		newDbConnectionArgsStruct.Threadlocals, ok = value.(bool)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("threadlocals must be of type bool")
		}
	}
	if value, ok := fullConfig["autorollback"]; ok {
		newDbConnectionArgsStruct.Autorollback, ok = value.(bool)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("autorollback must be of type bool")
		}
	}
	if value, ok := fullConfig["sslmode"]; ok {
		newDbConnectionArgsStruct.SslMode, ok = value.(string)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("sslmode must be of type string")
		}
	}
	if value, ok := fullConfig["sslrootcert"]; ok {
		newDbConnectionArgsStruct.SslRootCert, ok = value.(string)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("sslrootcert must be of type string")
		}
	}

	if value, ok := fullConfig["max_connections"]; ok {
		newDbConnectionArgsStruct.MaxConnections, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("max_connections must be a positive integer")
		}
	}

	if value, ok := fullConfig["stale_timeout"]; ok {
		newDbConnectionArgsStruct.StaleTimeout, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("stale_timeout must be a positive integer")
		}
	}

	if value, ok := fullConfig["keepalives"]; ok {
		newDbConnectionArgsStruct.KeepAlives, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("keepalives must be a positive integer")
		}
	}

	if value, ok := fullConfig["keepalives_idle"]; ok {
		newDbConnectionArgsStruct.KeepalivesIdle, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("keepalives_idle must be a positive integer")
		}
	}

	if value, ok := fullConfig["keepalives_interval"]; ok {
		newDbConnectionArgsStruct.KeepalivesInterval, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("keepalives_interval must be a positive integer")
		}
	}

	if value, ok := fullConfig["keepalives_count"]; ok {
		newDbConnectionArgsStruct.KeepalivesCount, ok = value.(int)
		if !ok {
			return newDbConnectionArgsStruct, errors.New("keepalives_count must be a positive integer")
		}
	}

	return newDbConnectionArgsStruct, nil
}

// NewSslStruct creates a new SslStruct
func NewSslStruct(fullConfig map[string]interface{}) (*SslStruct, error) {
	newSslStruct := &SslStruct{}
	defaults.Set(newSslStruct)

	if value, ok := fullConfig["ca"]; ok {
		newSslStruct.Ca, ok = value.(string)
		if !ok {
			return newSslStruct, errors.New("ca must be of type string")
		}
	}

	return newSslStruct, nil
}
