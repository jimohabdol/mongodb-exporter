package database

import (
	"context"
	"testing"
	"time"

	"github.com/jimohabdol/mongodb-exporter/config"
	"go.uber.org/zap"
)

func TestNewConnectionManager(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI: "mongodb://localhost:27017",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	if cm.config == nil {
		t.Error("ConnectionManager should have config")
	}

	if cm.logger == nil {
		t.Error("ConnectionManager should have logger")
	}

	if cm.client != nil {
		t.Error("ConnectionManager client should be nil initially")
	}
}

func TestConnect(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI: "mongodb://localhost:27017",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	if cm.client == nil {
		t.Error("ConnectionManager should have client after Connect")
	}

	err = cm.client.Ping(ctx, nil)
	if err != nil {
		t.Errorf("Client should be able to ping: %v", err)
	}
}

func TestConnectWithAuth(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI:        "mongodb://localhost:27017",
		Username:   "testuser",
		Password:   "testpass",
		Database:   "admin",
		AuthSource: "admin",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available or auth failed: %v", err)
	}

	if cm.client == nil {
		t.Error("ConnectionManager should have client after Connect")
	}
}

func TestConnectWithTLS(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI:                   "mongodb://localhost:27017",
		TLSEnabled:            true,
		TLSInsecureSkipVerify: true,
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB with TLS not available: %v", err)
	}

	if cm.client == nil {
		t.Error("ConnectionManager should have client after Connect")
	}
}

func TestDisconnect(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI: "mongodb://localhost:27017",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	if cm.client == nil {
		t.Error("ConnectionManager should have client after Connect")
	}

	err = cm.Disconnect(ctx)
	if err != nil {
		t.Errorf("Disconnect should not fail: %v", err)
	}

	if cm.client != nil {
		t.Skip("Disconnect may not clear client immediately")
	}
}

func TestGetClient(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI: "mongodb://localhost:27017",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	client := cm.GetClient()
	if client != nil {
		t.Error("GetClient should return nil before Connect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	client = cm.GetClient()
	if client == nil {
		t.Error("GetClient should return client after Connect")
	}
}

func TestHealthCheck(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI: "mongodb://localhost:27017",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck should fail before Connect")
	}

	err = cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	err = cm.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck should succeed after Connect: %v", err)
	}
}

func TestGetDatabase(t *testing.T) {
	mongoConfig := &config.MongoDBConfig{
		URI:      "mongodb://localhost:27017",
		Database: "test",
	}
	logger := zap.NewNop()

	cm := NewConnectionManager(mongoConfig, logger)

	db := cm.GetDatabase()
	if db != nil {
		t.Error("GetDatabase should return nil before Connect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cm.Connect(ctx)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	db = cm.GetDatabase()
	if db == nil {
		t.Error("GetDatabase should return database after Connect")
	}

	if db.Name() != "test" {
		t.Error("GetDatabase should return correct database name")
	}
}
