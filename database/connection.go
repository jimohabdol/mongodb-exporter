package database

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/jimohabdol/mongodb-exporter/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type ConnectionManager struct {
	client *mongo.Client
	logger *zap.Logger
	config *config.MongoDBConfig
}

func NewConnectionManager(cfg *config.MongoDBConfig, logger *zap.Logger) *ConnectionManager {
	return &ConnectionManager{
		logger: logger,
		config: cfg,
	}
}

func (cm *ConnectionManager) Connect(ctx context.Context) error {
	opts := options.Client().ApplyURI(cm.config.URI)

	opts.SetConnectTimeout(cm.config.ConnectionTimeout)
	opts.SetServerSelectionTimeout(cm.config.ServerSelectionTimeout)

	opts.SetMaxPoolSize(cm.config.MaxPoolSize)
	opts.SetMinPoolSize(cm.config.MinPoolSize)
	opts.SetMaxConnIdleTime(cm.config.MaxIdleTime)

	if cm.config.Username != "" && cm.config.Password != "" {
		credential := options.Credential{
			Username:   cm.config.Username,
			Password:   cm.config.Password,
			AuthSource: cm.config.AuthSource,
		}

		switch cm.config.AuthMechanism {
		case "SCRAM-SHA-1":
			credential.AuthMechanism = "SCRAM-SHA-1"
		case "SCRAM-SHA-256":
			credential.AuthMechanism = "SCRAM-SHA-256"
		case "MONGODB-X509":
			credential.AuthMechanism = "MONGODB-X509"
		case "PLAIN":
			credential.AuthMechanism = "PLAIN"
		case "GSSAPI":
			credential.AuthMechanism = "GSSAPI"
		default:
			credential.AuthMechanism = "SCRAM-SHA-256"
		}

		opts.SetAuth(credential)
	}

	if cm.config.TLSEnabled {
		tlsConfig, err := cm.buildTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	cm.client = client
	cm.logger.Info("Successfully connected to MongoDB",
		zap.String("uri", cm.config.URI),
		zap.String("database", cm.config.Database))

	return nil
}

func (cm *ConnectionManager) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cm.config.TLSInsecureSkipVerify,
	}

	if cm.config.TLSCAFile != "" {
		caCert, err := os.ReadFile(cm.config.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	if cm.config.TLSCertFile != "" && cm.config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cm.config.TLSCertFile, cm.config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func (cm *ConnectionManager) GetClient() *mongo.Client {
	return cm.client
}

func (cm *ConnectionManager) Disconnect(ctx context.Context) error {
	if cm.client != nil {
		if err := cm.client.Disconnect(ctx); err != nil {
			cm.logger.Error("Failed to disconnect from MongoDB", zap.Error(err))
			return err
		}
		cm.logger.Info("Disconnected from MongoDB")
	}
	return nil
}

func (cm *ConnectionManager) HealthCheck(ctx context.Context) error {
	if cm.client == nil {
		return fmt.Errorf("MongoDB client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return cm.client.Ping(ctx, nil)
}

func (cm *ConnectionManager) GetDatabase() *mongo.Database {
	if cm.client == nil {
		return nil
	}
	return cm.client.Database(cm.config.Database)
}
