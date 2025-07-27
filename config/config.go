package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MongoDB    MongoDBConfig    `yaml:"mongodb"`
	Server     ServerConfig     `yaml:"server"`
	Metrics    MetricsConfig    `yaml:"metrics"`
	Logging    LoggingConfig    `yaml:"logging"`
	Collectors CollectorsConfig `yaml:"collectors"`
}

type MongoDBConfig struct {
	URI                    string        `yaml:"uri" env:"MONGO_URI"`
	Username               string        `yaml:"username" env:"MONGO_USERNAME"`
	Password               string        `yaml:"password" env:"MONGO_PASSWORD"`
	Database               string        `yaml:"database" env:"MONGO_DATABASE"`
	AuthSource             string        `yaml:"auth_source" env:"MONGO_AUTH_SOURCE"`
	AuthMechanism          string        `yaml:"auth_mechanism" env:"MONGO_AUTH_MECHANISM"`
	TLSEnabled             bool          `yaml:"tls_enabled" env:"MONGO_TLS_ENABLED"`
	TLSInsecureSkipVerify  bool          `yaml:"tls_insecure_skip_verify" env:"MONGO_TLS_INSECURE_SKIP_VERIFY"`
	TLSCertFile            string        `yaml:"tls_cert_file" env:"MONGO_TLS_CERT_FILE"`
	TLSKeyFile             string        `yaml:"tls_key_file" env:"MONGO_TLS_KEY_FILE"`
	TLSCAFile              string        `yaml:"tls_ca_file" env:"MONGO_TLS_CA_FILE"`
	ConnectionTimeout      time.Duration `yaml:"connection_timeout" env:"MONGO_CONNECTION_TIMEOUT"`
	ServerSelectionTimeout time.Duration `yaml:"server_selection_timeout" env:"MONGO_SERVER_SELECTION_TIMEOUT"`
	MaxPoolSize            uint64        `yaml:"max_pool_size" env:"MONGO_MAX_POOL_SIZE"`
	MinPoolSize            uint64        `yaml:"min_pool_size" env:"MONGO_MIN_POOL_SIZE"`
	MaxIdleTime            time.Duration `yaml:"max_idle_time" env:"MONGO_MAX_IDLE_TIME"`
}

type ServerConfig struct {
	Port         string        `yaml:"port" env:"SERVER_PORT"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"SERVER_IDLE_TIMEOUT"`
}

type MetricsConfig struct {
	CollectionInterval time.Duration     `yaml:"collection_interval" env:"METRICS_COLLECTION_INTERVAL"`
	EnabledMetrics     []string          `yaml:"enabled_metrics" env:"METRICS_ENABLED"`
	DisabledMetrics    []string          `yaml:"disabled_metrics" env:"METRICS_DISABLED"`
	CustomLabels       map[string]string `yaml:"custom_labels" env:"METRICS_CUSTOM_LABELS"`
}

type LoggingConfig struct {
	Level      string `yaml:"level" env:"LOG_LEVEL"`
	Format     string `yaml:"format" env:"LOG_FORMAT"`
	OutputPath string `yaml:"output_path" env:"LOG_OUTPUT_PATH"`
}

type CollectorsConfig struct {
	CollStats      CollStatsConfig      `yaml:"collstats"`
	Profile        ProfileConfig        `yaml:"profile"`
	Sharding       ShardingConfig       `yaml:"sharding"`
	IndexStats     IndexStatsConfig     `yaml:"index_stats"`
	ConnectionPool ConnectionPoolConfig `yaml:"connection_pool"`
}

type CollStatsConfig struct {
	MonitoredCollections []string `yaml:"monitored_collections"`
}

type ProfileConfig struct {
	SlowOperationThreshold string `yaml:"slow_operation_threshold"`
	MaxEntriesPerCycle     int    `yaml:"max_entries_per_cycle"`
}

type ShardingConfig struct {
	CollectChunkDistribution bool `yaml:"collect_chunk_distribution"`
	CollectMigrationHistory  bool `yaml:"collect_migration_history"`
}

type IndexStatsConfig struct {
	CollectUsageStats       bool `yaml:"collect_usage_stats"`
	MaxIndexesPerCollection int  `yaml:"max_indexes_per_collection"`
}

type ConnectionPoolConfig struct {
	CollectPerHostMetrics    bool `yaml:"collect_per_host_metrics"`
	AnalyzeCurrentOperations bool `yaml:"analyze_current_operations"`
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	setDefaults(config)

	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func setDefaults(config *Config) {
	config.MongoDB.URI = "mongodb://localhost:27017"
	config.MongoDB.Database = "admin"
	config.MongoDB.AuthSource = "admin"
	config.MongoDB.AuthMechanism = "SCRAM-SHA-256"
	config.MongoDB.ConnectionTimeout = 10 * time.Second
	config.MongoDB.ServerSelectionTimeout = 30 * time.Second
	config.MongoDB.MaxPoolSize = 100
	config.MongoDB.MinPoolSize = 5
	config.MongoDB.MaxIdleTime = 30 * time.Minute

	config.Server.Port = "8080"
	config.Server.ReadTimeout = 30 * time.Second
	config.Server.WriteTimeout = 30 * time.Second
	config.Server.IdleTimeout = 60 * time.Second

	config.Metrics.CollectionInterval = 15 * time.Second

	config.Logging.Level = "info"
	config.Logging.Format = "json"
}

func loadFromFile(config *Config, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

func loadFromEnv(config *Config) error {
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		config.MongoDB.URI = uri
	}
	if username := os.Getenv("MONGO_USERNAME"); username != "" {
		config.MongoDB.Username = username
	}
	if password := os.Getenv("MONGO_PASSWORD"); password != "" {
		config.MongoDB.Password = password
	}
	if database := os.Getenv("MONGO_DATABASE"); database != "" {
		config.MongoDB.Database = database
	}
	if authSource := os.Getenv("MONGO_AUTH_SOURCE"); authSource != "" {
		config.MongoDB.AuthSource = authSource
	}
	if authMechanism := os.Getenv("MONGO_AUTH_MECHANISM"); authMechanism != "" {
		config.MongoDB.AuthMechanism = authMechanism
	}
	if tlsEnabled := os.Getenv("MONGO_TLS_ENABLED"); tlsEnabled != "" {
		if enabled, err := strconv.ParseBool(tlsEnabled); err == nil {
			config.MongoDB.TLSEnabled = enabled
		}
	}
	if tlsInsecureSkipVerify := os.Getenv("MONGO_TLS_INSECURE_SKIP_VERIFY"); tlsInsecureSkipVerify != "" {
		if skip, err := strconv.ParseBool(tlsInsecureSkipVerify); err == nil {
			config.MongoDB.TLSInsecureSkipVerify = skip
		}
	}
	if tlsCertFile := os.Getenv("MONGO_TLS_CERT_FILE"); tlsCertFile != "" {
		config.MongoDB.TLSCertFile = tlsCertFile
	}
	if tlsKeyFile := os.Getenv("MONGO_TLS_KEY_FILE"); tlsKeyFile != "" {
		config.MongoDB.TLSKeyFile = tlsKeyFile
	}
	if tlsCAFile := os.Getenv("MONGO_TLS_CA_FILE"); tlsCAFile != "" {
		config.MongoDB.TLSCAFile = tlsCAFile
	}
	if connectionTimeout := os.Getenv("MONGO_CONNECTION_TIMEOUT"); connectionTimeout != "" {
		if timeout, err := time.ParseDuration(connectionTimeout); err == nil {
			config.MongoDB.ConnectionTimeout = timeout
		}
	}
	if serverSelectionTimeout := os.Getenv("MONGO_SERVER_SELECTION_TIMEOUT"); serverSelectionTimeout != "" {
		if timeout, err := time.ParseDuration(serverSelectionTimeout); err == nil {
			config.MongoDB.ServerSelectionTimeout = timeout
		}
	}
	if maxPoolSize := os.Getenv("MONGO_MAX_POOL_SIZE"); maxPoolSize != "" {
		if size, err := strconv.ParseUint(maxPoolSize, 10, 64); err == nil {
			config.MongoDB.MaxPoolSize = size
		}
	}
	if minPoolSize := os.Getenv("MONGO_MIN_POOL_SIZE"); minPoolSize != "" {
		if size, err := strconv.ParseUint(minPoolSize, 10, 64); err == nil {
			config.MongoDB.MinPoolSize = size
		}
	}
	if maxIdleTime := os.Getenv("MONGO_MAX_IDLE_TIME"); maxIdleTime != "" {
		if timeout, err := time.ParseDuration(maxIdleTime); err == nil {
			config.MongoDB.MaxIdleTime = timeout
		}
	}

	if port := os.Getenv("SERVER_PORT"); port != "" {
		config.Server.Port = port
	}
	if readTimeout := os.Getenv("SERVER_READ_TIMEOUT"); readTimeout != "" {
		if timeout, err := time.ParseDuration(readTimeout); err == nil {
			config.Server.ReadTimeout = timeout
		}
	}
	if writeTimeout := os.Getenv("SERVER_WRITE_TIMEOUT"); writeTimeout != "" {
		if timeout, err := time.ParseDuration(writeTimeout); err == nil {
			config.Server.WriteTimeout = timeout
		}
	}
	if idleTimeout := os.Getenv("SERVER_IDLE_TIMEOUT"); idleTimeout != "" {
		if timeout, err := time.ParseDuration(idleTimeout); err == nil {
			config.Server.IdleTimeout = timeout
		}
	}

	if collectionInterval := os.Getenv("METRICS_COLLECTION_INTERVAL"); collectionInterval != "" {
		if interval, err := time.ParseDuration(collectionInterval); err == nil {
			config.Metrics.CollectionInterval = interval
		}
	}
	if enabledMetrics := os.Getenv("METRICS_ENABLED"); enabledMetrics != "" {
		config.Metrics.EnabledMetrics = strings.Split(enabledMetrics, ",")
	}
	if disabledMetrics := os.Getenv("METRICS_DISABLED"); disabledMetrics != "" {
		config.Metrics.DisabledMetrics = strings.Split(disabledMetrics, ",")
	}

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
	if outputPath := os.Getenv("LOG_OUTPUT_PATH"); outputPath != "" {
		config.Logging.OutputPath = outputPath
	}

	return nil
}

func validateConfig(config *Config) error {
	if config.MongoDB.URI == "" {
		return fmt.Errorf("MongoDB URI is required")
	}

	if config.MongoDB.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}

	if config.MongoDB.ServerSelectionTimeout <= 0 {
		return fmt.Errorf("server selection timeout must be positive")
	}

	if config.MongoDB.MaxPoolSize < config.MongoDB.MinPoolSize {
		return fmt.Errorf("max pool size cannot be less than min pool size")
	}

	if config.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if config.Server.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}

	if config.Server.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	if config.Server.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive")
	}

	if config.Metrics.CollectionInterval <= 0 {
		return fmt.Errorf("collection interval must be positive")
	}

	return nil
}
