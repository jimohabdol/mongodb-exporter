# MongoDB Exporter

A production-ready Prometheus exporter for MongoDB metrics, designed to provide comprehensive monitoring capabilities for MongoDB instances.

[![Go Report Card](https://goreportcard.com/badge/github.com/jimohabdol/mongodb-exporter)](https://goreportcard.com/report/github.com/jimohabdol/mongodb-exporter)
[![Go Version](https://img.shields.io/github/go-mod/go-version/jimohabdol/mongodb-exporter)](https://github.com/jimohabdol/mongodb-exporter)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Features

- **Comprehensive Metrics Collection**: Collects server status, replica set health, WiredTiger storage engine metrics, lock statistics, index usage, and more
- **Production Ready**: Optimized for production environments with configurable timeouts and resource limits
- **Low Impact**: Designed to minimize impact on MongoDB performance
- **Flexible Configuration**: Support for custom labels, metric filtering, and collector-specific settings
- **Grafana Integration**: Includes pre-configured Grafana dashboard for visualization

## Quick Start

### Prerequisites

- Go 1.19 or higher
- MongoDB 4.0 or higher
- Docker (optional, for containerized deployment)

### Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/jimohabdol/mongodb-exporter.git
   cd mongo-exporter
   ```

2. **Build the exporter**:
   ```bash
   go mod tidy
   go build -o mongo-exporter main.go
   ```

3. **Configure the exporter**:
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your MongoDB connection details
   ```

4. **Run the exporter**:
   ```bash
   ./mongo-exporter -config config.yaml
   ```

### Docker Deployment

1. **Build the Docker image**:
   ```bash
   docker build -t mongo-exporter .
   ```

2. **Run with Docker Compose**:
   ```bash
   docker-compose up -d
   ```

## Configuration

The exporter uses YAML configuration files. See `config.example.yaml` for a complete example.

### Key Configuration Sections

- **MongoDB Connection**: Connection string, authentication, and connection pool settings
- **Server Settings**: HTTP server configuration and timeouts
- **Metrics**: Collection intervals and enabled/disabled metrics
- **Collectors**: Collector-specific configurations
- **Logging**: Log level and format settings

### Environment Variables

All configuration options can be overridden using environment variables. See `config/config.go` for the complete list.

## Metrics

The exporter collects metrics from multiple MongoDB collectors:

- **Server Status**: Uptime, connections, memory usage, network traffic
- **Replica Set**: Health status, member states, replication lag
- **WiredTiger**: Storage engine statistics and performance metrics
- **Locks**: Lock contention and deadlock detection
- **Index Statistics**: Index usage, size, and performance metrics
- **Collection Statistics**: Document counts, storage sizes, operation latencies
- **Query Executor**: Query performance and execution statistics
- **Connection Pool**: Connection pool utilization and metrics
- **Cursors**: Active cursor statistics
- **Profile**: Slow query analysis and profiling data

## Grafana Dashboard

A pre-configured Grafana dashboard is included in the `grafana/` directory. The dashboard provides:

- MongoDB instance overview
- Connection and memory metrics
- Replica set health monitoring
- Index usage and performance
- Collection statistics
- Lock contention analysis

## Development

### Project Structure

```
mongo-exporter/
├── collector/          # Metric collectors
├── config/            # Configuration management
├── database/          # Database connection handling
├── server/            # HTTP server implementation
├── grafana/           # Grafana dashboard files
├── main.go           # Application entry point
├── Dockerfile        # Docker configuration
├── docker-compose.yml # Docker Compose setup
└── config.example.yaml # Example configuration
```

### Building

```bash
# Build for current platform
go build -o mongo-exporter main.go

GOOS=linux GOARCH=amd64 go build -o mongo-exporter main.go

go build -ldflags "-X main.version=1.0.0 -X main.buildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" -o mongo-exporter main.go
```

### Testing

```bash

go test ./...

go test -cover ./...


go test ./collector -v
```

## Security Considerations

- **Authentication**: Always use authentication when connecting to MongoDB
- **Network Security**: Use TLS/SSL for encrypted connections
- **Access Control**: Limit MongoDB user permissions to read-only access
- **Configuration**: Never commit configuration files with sensitive credentials
- **Firewall**: Restrict access to the exporter's HTTP endpoint

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:

1. Check the documentation in the `docs/` directory
2. Review existing issues on GitHub
3. Create a new issue with detailed information about your problem

## Changelog

### Version 1.0.0
- Initial release
- Comprehensive MongoDB metrics collection
- Grafana dashboard integration
- Production-ready configuration
- Docker support
