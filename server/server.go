package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jimohabdol/mongodb-exporter/collector"
	"github.com/jimohabdol/mongodb-exporter/config"
	"github.com/jimohabdol/mongodb-exporter/database"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server struct {
	config            *config.Config
	logger            *zap.Logger
	connectionManager *database.ConnectionManager
	collectorManager  *collector.CollectorManager
	server            *http.Server
	registry          *prometheus.Registry
}

func NewServer(cfg *config.Config, logger *zap.Logger, connManager *database.ConnectionManager) *Server {
	registry := prometheus.NewRegistry()

	collectorConfig := collector.CollectorConfig{
		CustomLabels:    cfg.Metrics.CustomLabels,
		EnabledMetrics:  cfg.Metrics.EnabledMetrics,
		DisabledMetrics: cfg.Metrics.DisabledMetrics,
		Collectors:      make(map[string]interface{}),
	}

	// Add collector-specific configurations
	if len(cfg.Collectors.CollStats.MonitoredCollections) > 0 {
		collectorConfig.Collectors["collstats"] = map[string]interface{}{
			"monitored_collections": cfg.Collectors.CollStats.MonitoredCollections,
		}
	}

	collectorManager := collector.NewCollectorManager(connManager.GetClient(), logger, collectorConfig)

	return &Server{
		config:            cfg,
		logger:            logger,
		connectionManager: connManager,
		collectorManager:  collectorManager,
		registry:          registry,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.collectorManager.InitializeCollectors(); err != nil {
		return fmt.Errorf("failed to initialize collectors: %w", err)
	}

	if err := s.registry.Register(s.collectorManager.GetCollector()); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}

	s.server = &http.Server{
		Addr:         ":" + s.config.Server.Port,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
		Handler:      s.createHandler(),
	}

	s.logger.Info("Starting MongoDB exporter server",
		zap.String("port", s.config.Server.Port),
		zap.Duration("read_timeout", s.config.Server.ReadTimeout),
		zap.Duration("write_timeout", s.config.Server.WriteTimeout))

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping MongoDB exporter server")

	// Shutdown collector manager first
	s.collectorManager.Shutdown()

	// Shutdown HTTP server
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Failed to shutdown server gracefully", zap.Error(err))
			return err
		}
	}

	// Clear registry to free memory
	if s.registry != nil {
		s.registry = prometheus.NewRegistry()
	}

	s.logger.Info("MongoDB exporter server stopped")
	return nil
}

func (s *Server) createHandler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/metrics", s.addMiddleware(promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})))
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/", s.rootHandler)

	return s.addMiddleware(mux)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.connectionManager.HealthCheck(r.Context()); err != nil {
		s.logger.Error("Health check failed", zap.Error(err))
		http.Error(w, "Health check failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>MongoDB Exporter</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        h1 { color: #333; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .endpoint h3 { margin: 0 0 10px 0; color: #666; }
        .endpoint p { margin: 5px 0; }
        a { color: #007cba; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>MongoDB Exporter</h1>
        <p>A Prometheus exporter for MongoDB metrics.</p>
        
        <div class="endpoint">
            <h3>Available Endpoints:</h3>
            <p><strong>Metrics:</strong> <a href="/metrics">/metrics</a> - Prometheus metrics</p>
            <p><strong>Health:</strong> <a href="/health">/health</a> - Health check endpoint</p>
        </div>
        
        <div class="endpoint">
            <h3>Usage:</h3>
            <p>Add this exporter to your Prometheus configuration:</p>
            <pre>scrape_configs:
  - job_name: 'mongodb'
    static_configs:
      - targets: ['localhost:8080']</pre>
        </div>
    </div>
</body>
</html>`))
}

func (s *Server) addMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		s.logger.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()))

		handler.ServeHTTP(w, r)

		s.logger.Info("HTTP response",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)))
	})
}

func (s *Server) GetRegistry() *prometheus.Registry {
	return s.registry
}
