package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jimohabdol/mongodb-exporter/config"
	"github.com/jimohabdol/mongodb-exporter/database"
	"go.uber.org/zap"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: "8080",
		},
	}
	logger := zap.NewNop()
	connManager := &database.ConnectionManager{}

	server := NewServer(cfg, logger, connManager)

	if server.config == nil {
		t.Error("Server should have config")
	}

	if server.logger == nil {
		t.Error("Server should have logger")
	}

	if server.connectionManager == nil {
		t.Error("Server should have connection manager")
	}

	if server.registry == nil {
		t.Error("Server should have registry")
	}
}

func TestStart(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: "0",
		},
	}
	logger := zap.NewNop()
	connManager := &database.ConnectionManager{}

	server := NewServer(cfg, logger, connManager)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := server.Start(ctx)
	if err != nil {
		t.Errorf("Server should start: %v", err)
	}
}

func TestGetRegistry(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: "0",
		},
	}
	logger := zap.NewNop()
	connManager := &database.ConnectionManager{}

	server := NewServer(cfg, logger, connManager)

	registry := server.GetRegistry()
	if registry == nil {
		t.Error("GetRegistry should return registry")
	}
}

type mockResponseWriter struct {
	statusCode int
	body       string
	header     http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.body += string(data)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}
