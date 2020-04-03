package quayconfig

import (
	"fmt"
)

// toConnectionString creates a Database connection string
func (config *DatabaseConfig) ToConnectionString() (string, error) {

	// TODO: validate fields
	format := "postgresql://%s:%s@%s/%s"
	connectionString := fmt.Sprintf(format, config.Username, config.Password, config.Host, config.Name)
	return connectionString, nil

}
