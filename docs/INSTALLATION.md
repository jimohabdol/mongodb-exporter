# Installation Guide

This guide provides detailed instructions for installing and configuring the MongoDB Exporter.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Go Version**: 1.19 or higher
- **MongoDB Version**: 4.0 or higher
- **Memory**: Minimum 512MB RAM
- **Disk Space**: 100MB for the exporter binary

### Required Software

1. **Go Programming Language**
   ```bash
   # Download and install Go from https://golang.org/dl/
   # Verify installation
   go version
   ```

2. **Git** (for cloning the repository)
   ```bash
   # Install git
   # Ubuntu/Debian
   sudo apt-get install git
   
   # macOS
   brew install git
   
   # Windows
   # Download from https://git-scm.com/download/win
   ```

## Installation Methods

### Method 1: From Source (Recommended)

1. **Clone the Repository**
   ```bash
   git clone https://github.com/jimohabdol/mongodb-exporter.git
   cd mongo-exporter
   ```

2. **Install Dependencies**
   ```bash
   go mod tidy
   ```

3. **Build the Exporter**
   ```bash
   go build -o mongo-exporter main.go
   ```

4. **Verify the Build**
   ```bash
   ./mongo-exporter --version
   ```

### Method 2: Using Docker

1. **Build Docker Image**
   ```bash
   docker build -t mongo-exporter .
   ```

2. **Run with Docker**
   ```bash
   docker run -d \
     --name mongo-exporter \
     -p 9216:9216 \
     -v $(pwd)/config.yaml:/app/config.yaml \
     mongo-exporter
   ```

### Method 3: Using Docker Compose

1. **Start the Stack**
   ```bash
   docker-compose up -d
   ```

2. **Verify Services**
   ```bash
   docker-compose ps
   ```

## Configuration

### Basic Configuration

1. **Copy Example Configuration**
   ```bash
   cp config.example.yaml config.yaml
   ```

2. **Edit Configuration**
   ```bash
   nano config.yaml
   ```

3. **Basic Configuration Example**
   ```yaml
   mongodb:
     uri: "mongodb://localhost:27017"
     timeout: "10s"
   
   server:
     port: 9216
     host: "0.0.0.0"
   
   metrics:
     collection_interval: "15s"
     enabled_metrics:
       - "server_status"
       - "replica_set_status"
       - "wiredtiger"
   
   logging:
     level: "info"
     format: "json"
   ```

### Advanced Configuration

#### MongoDB Authentication

```yaml
mongodb:
  uri: "mongodb://username:password@localhost:27017/database"
  username: "monitoring_user"
  password: "secure_password"
  auth_source: "admin"
  auth_mechanism: "SCRAM-SHA-256"
```

#### TLS/SSL Configuration

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  tls_enabled: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
  tls_ca_file: "/path/to/ca.pem"
  tls_insecure_skip_verify: false
```

#### Connection Pool Settings

```yaml
mongodb:
  max_pool_size: 100
  min_pool_size: 5
  max_idle_time: "30m"
  connection_timeout: "10s"
  server_selection_timeout: "30s"
```

## Running the Exporter

### Development Mode

```bash
# Run with debug logging
./mongo-exporter -config config.yaml

# Run with specific log level
LOG_LEVEL=debug ./mongo-exporter -config config.yaml
```

### Production Mode

```bash
# Run as a service
nohup ./mongo-exporter -config config.yaml > exporter.log 2>&1 &

# Run with systemd (create service file)
sudo systemctl start mongo-exporter
sudo systemctl enable mongo-exporter
```

### Docker Mode

```bash
# Run with custom configuration
docker run -d \
  --name mongo-exporter \
  -p 9216:9216 \
  -v /path/to/config.yaml:/app/config.yaml \
  mongo-exporter

# Run with environment variables
docker run -d \
  --name mongo-exporter \
  -p 9216:9216 \
  -e MONGO_URI="mongodb://localhost:27017" \
  -e SERVER_PORT="9216" \
  mongo-exporter
```

## Verification

### Check Exporter Status

1. **HTTP Endpoint**
   ```bash
   curl http://localhost:9216/metrics
   ```

2. **Health Check**
   ```bash
   curl http://localhost:9216/health
   ```

3. **Version Information**
   ```bash
   curl http://localhost:9216/version
   ```

### Verify Metrics

```bash
# Check for MongoDB metrics
curl http://localhost:9216/metrics | grep mongodb

# Check specific metrics
curl http://localhost:9216/metrics | grep "mongodb_connections"
curl http://localhost:9216/metrics | grep "mongodb_memory"
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Verify MongoDB is running
   - Check connection string
   - Ensure network connectivity

2. **Authentication Failed**
   - Verify username/password
   - Check auth_source setting
   - Ensure user has read permissions

3. **No Metrics Returned**
   - Check enabled_metrics configuration
   - Verify MongoDB version compatibility
   - Review log files for errors

### Log Analysis

```bash
# View logs
tail -f exporter.log

# Search for errors
grep "ERROR" exporter.log

# Check specific collector logs
grep "server_status" exporter.log
```

### Performance Tuning

1. **Reduce Collection Interval**
   ```yaml
   metrics:
     collection_interval: "30s"  # Increase for high-load systems
   ```

2. **Limit Monitored Collections**
   ```yaml
   collectors:
     collstats:
       monitored_collections:
         - "important_collection_1"
         - "important_collection_2"
   ```

3. **Optimize Connection Pool**
   ```yaml
   mongodb:
     max_pool_size: 50
     min_pool_size: 2
   ```

## Next Steps

After successful installation:

1. **Configure Prometheus** to scrape the exporter
2. **Import Grafana Dashboard** from the `grafana/` directory
3. **Set up Alerting** based on collected metrics
4. **Monitor Performance** and adjust configuration as needed

For more information, see the [Configuration Guide](CONFIGURATION.md) and [Metrics Reference](METRICS_REFERENCE.md). 