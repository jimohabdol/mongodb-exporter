package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jimohabdol/mongodb-exporter/config"
	"github.com/jimohabdol/mongodb-exporter/database"
	"github.com/jimohabdol/mongodb-exporter/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("MongoDB Exporter v%s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger, err := setupLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting MongoDB Exporter",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	connManager := database.NewConnectionManager(&cfg.MongoDB, logger)

	if err := connManager.Connect(ctx); err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}

	srv := server.NewServer(cfg, logger, connManager)
	if err := srv.Start(ctx); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}

	logger.Info("MongoDB Exporter started successfully",
		zap.String("port", cfg.Server.Port),
		zap.String("mongodb_uri", cfg.MongoDB.URI))

	<-sigChan
	logger.Info("Received shutdown signal, starting graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		logger.Error("Failed to stop server gracefully", zap.Error(err))
	}

	if err := connManager.Disconnect(shutdownCtx); err != nil {
		logger.Error("Failed to disconnect from MongoDB", zap.Error(err))
	}

	logger.Info("MongoDB Exporter shutdown complete")
}

func setupLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)

	if cfg.OutputPath != "" {
		config.OutputPaths = []string{cfg.OutputPath}
	} else {
		config.OutputPaths = []string{"stdout"}
	}
	config.ErrorOutputPaths = []string{"stderr"}

	if cfg.Format == "console" {
		config.Encoding = "console"
	} else {
		config.Encoding = "json"
	}

	return config.Build()
}
