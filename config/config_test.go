package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig("")
	if err != nil {
		t.Errorf("LoadConfig failed: %v", err)
	}

	if config.MongoDB.URI != "mongodb://localhost:27017" {
		t.Error("Default MongoDB URI should be set")
	}

	if config.Server.Port != "8080" {
		t.Error("Default server port should be set")
	}

	if config.Metrics.CollectionInterval != 15*time.Second {
		t.Error("Default collection interval should be set")
	}

	if config.Logging.Level != "info" {
		t.Error("Default logging level should be set")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tempFile := "test_config.yaml"
	defer os.Remove(tempFile)

	configContent := `
mongodb:
  uri: "mongodb://test:27017"
  timeout: "30s"
server:
  port: 9090
metrics:
  collection_interval: "30s"
logging:
  level: "debug"
`

	err := os.WriteFile(tempFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadConfig(tempFile)
	if err != nil {
		t.Errorf("LoadConfig from file failed: %v", err)
	}

	if config.MongoDB.URI != "mongodb://test:27017" {
		t.Error("MongoDB URI should be loaded from file")
	}

	if config.Server.Port != "9090" {
		t.Error("Server port should be loaded from file")
	}

	if config.Metrics.CollectionInterval != 30*time.Second {
		t.Error("Collection interval should be loaded from file")
	}

	if config.Logging.Level != "debug" {
		t.Error("Logging level should be loaded from file")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://env:27017")
	os.Setenv("SERVER_PORT", "9091")
	os.Setenv("LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("MONGO_URI")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	config, err := LoadConfig("")
	if err != nil {
		t.Errorf("LoadConfig with env vars failed: %v", err)
	}

	if config.MongoDB.URI != "mongodb://env:27017" {
		t.Error("MongoDB URI should be loaded from environment")
	}

	if config.Server.Port != "9091" {
		t.Error("Server port should be loaded from environment")
	}

	if config.Logging.Level != "warn" {
		t.Error("Logging level should be loaded from environment")
	}
}

func TestValidateConfig(t *testing.T) {
	config := &Config{
		MongoDB: MongoDBConfig{
			URI:                    "mongodb://localhost:27017",
			ConnectionTimeout:      10 * time.Second,
			ServerSelectionTimeout: 30 * time.Second,
		},
		Server: ServerConfig{
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Metrics: MetricsConfig{
			CollectionInterval: 15 * time.Second,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("Valid config should not return error: %v", err)
	}
}

func TestValidateConfigInvalid(t *testing.T) {
	config := &Config{
		MongoDB: MongoDBConfig{
			URI: "",
		},
		Server: ServerConfig{
			Port: "",
		},
		Metrics: MetricsConfig{
			CollectionInterval: 0,
		},
		Logging: LoggingConfig{
			Level: "",
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Invalid config should return error")
	}
}

func TestSetDefaults(t *testing.T) {
	config := &Config{}
	setDefaults(config)

	if config.MongoDB.URI != "mongodb://localhost:27017" {
		t.Error("Default MongoDB URI should be set")
	}

	if config.MongoDB.Database != "admin" {
		t.Error("Default MongoDB database should be set")
	}

	if config.MongoDB.AuthSource != "admin" {
		t.Error("Default auth source should be set")
	}

	if config.MongoDB.AuthMechanism != "SCRAM-SHA-256" {
		t.Error("Default auth mechanism should be set")
	}

	if config.MongoDB.ConnectionTimeout != 10*time.Second {
		t.Error("Default connection timeout should be set")
	}

	if config.MongoDB.ServerSelectionTimeout != 30*time.Second {
		t.Error("Default server selection timeout should be set")
	}

	if config.MongoDB.MaxPoolSize != 100 {
		t.Error("Default max pool size should be set")
	}

	if config.MongoDB.MinPoolSize != 5 {
		t.Error("Default min pool size should be set")
	}

	if config.MongoDB.MaxIdleTime != 30*time.Minute {
		t.Error("Default max idle time should be set")
	}

	if config.Server.Port != "8080" {
		t.Error("Default server port should be set")
	}

	if config.Server.ReadTimeout != 30*time.Second {
		t.Error("Default read timeout should be set")
	}

	if config.Server.WriteTimeout != 30*time.Second {
		t.Error("Default write timeout should be set")
	}

	if config.Server.IdleTimeout != 60*time.Second {
		t.Error("Default idle timeout should be set")
	}

	if config.Metrics.CollectionInterval != 15*time.Second {
		t.Error("Default collection interval should be set")
	}

	if config.Logging.Level != "info" {
		t.Error("Default logging level should be set")
	}

	if config.Logging.Format != "json" {
		t.Error("Default logging format should be set")
	}
}
