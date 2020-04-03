package quayconfig

type ConfigFileRedis struct {
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
}

type ConfigFile struct {
	DatabaseURI          string          `yaml:"DB_URI"`
	Redis                ConfigFileRedis `yaml:"USER_EVENTS_REDIS"`
	Hostname             string          `yaml:"SERVER_HOSTNAME"`
	NotManagedByOperator map[string]interface{}
}

// DatabaseConfig contains the information needed to configure Quay's database
// connection.
type DatabaseConfig struct {
	Host     string
	Username string
	Password string
	Name     string
}

// RedisConfig contains the information needed to configure Quay's redis
// connection.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
}

type InfrastructureConfig struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Hostname string
}
