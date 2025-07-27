# Configuration Guide

This guide provides detailed information about configuring the MongoDB Exporter.

## Configuration File Structure

The MongoDB Exporter uses YAML configuration files. The configuration is divided into several sections:

```yaml
mongodb:
  # MongoDB connection settings

server:
  # HTTP server configuration

metrics:
  # Metrics collection settings

logging:
  # Logging configuration

collectors:
  # Collector-specific settings
```

## MongoDB Configuration

### Basic Connection

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  timeout: "10s"
```

### Authentication

```yaml
mongodb:
  uri: "mongodb://username:password@localhost:27017/database"
  username: "monitoring_user"
  password: "secure_password"
  database: "admin"
  auth_source: "admin"
  auth_mechanism: "SCRAM-SHA-256"
```

### TLS/SSL Configuration

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  tls_enabled: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
  tls_ca_file: "/path/to/ca.pem"
  tls_insecure_skip_verify: false
```

### Connection Pool Settings

```yaml
mongodb:
  max_pool_size: 100
  min_pool_size: 5
  max_idle_time: "30m"
  connection_timeout: "10s"
  server_selection_timeout: "30s"
```

## Server Configuration

### Basic Server Settings

```yaml
server:
  port: 9216
  host: "0.0.0.0"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"
```

### Security Settings

```yaml
server:
  port: 9216
  host: "127.0.0.1"  # Bind to localhost only
  tls_enabled: true
  tls_cert_file: "/path/to/server.crt"
  tls_key_file: "/path/to/server.key"
```

## Metrics Configuration

### Basic Metrics Settings

```yaml
metrics:
  collection_interval: "15s"
  enabled_metrics:
    - "server_status"
    - "replica_set_status"
    - "wiredtiger"
    - "locks"
    - "index_stats"
    - "storage_stats"
    - "query_executor"
    - "operation_metrics"
    - "collstats"
    - "cursors"
    - "profile"
    - "connection_pool"
    - "compatibility"
    - "sharding"
  disabled_metrics:
    - "profile"  # Disable specific metrics
  custom_labels:
    instance: "mongodb-01"
    environment: "production"
    datacenter: "us-east-1"
```

### Metric Filtering

```yaml
metrics:
  enabled_metrics:
    - "server_status"      # Basic server metrics
    - "replica_set_status" # Replica set health
    - "wiredtiger"        # Storage engine metrics
    - "locks"             # Lock statistics
    - "index_stats"       # Index usage metrics
    - "storage_stats"     # Database/collection storage
    - "query_executor"    # Query performance
    - "operation_metrics" # Operation counters
    - "collstats"         # Collection statistics
    - "cursors"           # Cursor metrics
    - "profile"           # Slow query profiling
    - "connection_pool"   # Connection pool metrics
    - "compatibility"     # Version 1 compatibility
    - "sharding"          # Sharding metrics
```

## Logging Configuration

### Basic Logging

```yaml
logging:
  level: "info"
  format: "json"
  output_path: ""  # Empty for stdout
```

### Advanced Logging

```yaml
logging:
  level: "debug"
  format: "console"
  output_path: "/var/log/mongo-exporter.log"
```

### Log Levels

- `debug`: Detailed debug information
- `info`: General information messages
- `warn`: Warning messages
- `error`: Error messages only

## Collector Configuration

### Collection Statistics

```yaml
collectors:
  collstats:
    monitored_collections:
      - "*"  # Monitor all collections
      # Or specify specific collections:
      # - "users"
      # - "orders"
      # - "products"
```

### Profile Configuration

```yaml
collectors:
  profile:
    slow_operation_threshold: "100ms"
    max_entries_per_cycle: 500
```

### Sharding Configuration

```yaml
collectors:
  sharding:
    collect_chunk_distribution: true
    collect_migration_history: true
```

### Index Statistics

```yaml
collectors:
  index_stats:
    collect_usage_stats: true
    max_indexes_per_collection: 100
```

### Connection Pool

```yaml
collectors:
  connection_pool:
    collect_per_host_metrics: true
    analyze_current_operations: true
```

## Environment Variables

All configuration options can be overridden using environment variables:

### MongoDB Environment Variables

```bash
export MONGO_URI="mongodb://localhost:27017"
export MONGO_USERNAME="monitoring_user"
export MONGO_PASSWORD="secure_password"
export MONGO_DATABASE="admin"
export MONGO_AUTH_SOURCE="admin"
export MONGO_AUTH_MECHANISM="SCRAM-SHA-256"
export MONGO_TLS_ENABLED="true"
export MONGO_MAX_POOL_SIZE="100"
export MONGO_MIN_POOL_SIZE="5"
export MONGO_CONNECTION_TIMEOUT="10s"
export MONGO_SERVER_SELECTION_TIMEOUT="30s"
export MONGO_MAX_IDLE_TIME="30m"
```

### Server Environment Variables

```bash
export SERVER_PORT="9216"
export SERVER_READ_TIMEOUT="30s"
export SERVER_WRITE_TIMEOUT="30s"
export SERVER_IDLE_TIMEOUT="60s"
```

### Metrics Environment Variables

```bash
export METRICS_COLLECTION_INTERVAL="15s"
export METRICS_ENABLED="server_status,replica_set_status,wiredtiger"
export METRICS_DISABLED="profile"
export METRICS_CUSTOM_LABELS="instance=mongodb-01,environment=production"
```

### Logging Environment Variables

```bash
export LOG_LEVEL="info"
export LOG_FORMAT="json"
export LOG_OUTPUT_PATH="/var/log/mongo-exporter.log"
```

## Configuration Examples

### Development Configuration

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  timeout: "5s"

server:
  port: 9216
  host: "127.0.0.1"

metrics:
  collection_interval: "30s"
  enabled_metrics:
    - "server_status"
    - "replica_set_status"
    - "wiredtiger"
  custom_labels:
    environment: "development"

logging:
  level: "debug"
  format: "console"
```

### Production Configuration

```yaml
mongodb:
  uri: "mongodb://monitoring_user:password@mongodb.example.com:27017/admin"
  auth_source: "admin"
  auth_mechanism: "SCRAM-SHA-256"
  tls_enabled: true
  tls_cert_file: "/etc/ssl/certs/mongodb.crt"
  tls_key_file: "/etc/ssl/private/mongodb.key"
  tls_ca_file: "/etc/ssl/certs/ca.crt"
  max_pool_size: 50
  min_pool_size: 2
  connection_timeout: "10s"
  server_selection_timeout: "30s"

server:
  port: 9216
  host: "0.0.0.0"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"

metrics:
  collection_interval: "30s"
  enabled_metrics:
    - "server_status"
    - "replica_set_status"
    - "wiredtiger"
    - "locks"
    - "index_stats"
    - "storage_stats"
    - "query_executor"
    - "operation_metrics"
    - "collstats"
    - "cursors"
    - "connection_pool"
    - "compatibility"
    - "sharding"
  custom_labels:
    instance: "mongodb-prod-01"
    environment: "production"
    datacenter: "us-east-1"
    cluster: "main"

logging:
  level: "info"
  format: "json"
  output_path: "/var/log/mongo-exporter.log"

collectors:
  collstats:
    monitored_collections:
      - "users"
      - "orders"
      - "products"
      - "analytics"
  profile:
    slow_operation_threshold: "100ms"
    max_entries_per_cycle: 1000
  sharding:
    collect_chunk_distribution: true
    collect_migration_history: true
  index_stats:
    collect_usage_stats: true
    max_indexes_per_collection: 50
  connection_pool:
    collect_per_host_metrics: true
    analyze_current_operations: true
```

### High-Performance Configuration

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  max_pool_size: 200
  min_pool_size: 10
  connection_timeout: "5s"
  server_selection_timeout: "15s"

server:
  port: 9216
  host: "0.0.0.0"
  read_timeout: "15s"
  write_timeout: "15s"

metrics:
  collection_interval: "60s"  # Less frequent collection
  enabled_metrics:
    - "server_status"
    - "replica_set_status"
    - "wiredtiger"
    - "locks"
    - "index_stats"
    - "storage_stats"
    - "query_executor"
    - "operation_metrics"
    - "connection_pool"
    - "compatibility"
  disabled_metrics:
    - "collstats"    # Disable expensive collectors
    - "profile"
    - "cursors"
    - "sharding"

logging:
  level: "warn"
  format: "json"

collectors:
  index_stats:
    collect_usage_stats: false  # Disable expensive operations
    max_indexes_per_collection: 20
```

## Configuration Validation

### Command Line Validation

```bash
# Validate configuration file
./mongo-exporter -config config.yaml --validate

# Test configuration without starting
./mongo-exporter -config config.yaml --dry-run
```

### Configuration Testing

```bash
# Test MongoDB connection
./mongo-exporter -config config.yaml --test-connection

# Test metrics collection
./mongo-exporter -config config.yaml --test-metrics
```

## Best Practices

### Security

1. **Use Authentication**: Always use MongoDB authentication
2. **TLS/SSL**: Enable TLS for encrypted connections
3. **Network Security**: Bind to specific interfaces
4. **Access Control**: Use read-only MongoDB users
5. **Configuration Files**: Never commit sensitive configuration

### Performance

1. **Collection Interval**: Adjust based on system load
2. **Connection Pool**: Optimize pool size for your workload
3. **Metric Filtering**: Disable unnecessary collectors
4. **Collection Monitoring**: Limit monitored collections
5. **Timeout Settings**: Set appropriate timeouts

### Monitoring

1. **Log Levels**: Use appropriate log levels for production
2. **Custom Labels**: Add meaningful labels for identification
3. **Health Checks**: Monitor exporter health
4. **Resource Usage**: Monitor exporter resource consumption
5. **Error Handling**: Set up alerts for configuration errors

## Troubleshooting

### Common Configuration Issues

1. **Connection Timeouts**
   - Increase `connection_timeout` and `server_selection_timeout`
   - Check network connectivity
   - Verify MongoDB is running

2. **Authentication Errors**
   - Verify username/password
   - Check `auth_source` setting
   - Ensure user has read permissions

3. **No Metrics Returned**
   - Check `enabled_metrics` configuration
   - Verify MongoDB version compatibility
   - Review log files for errors

4. **High Resource Usage**
   - Reduce `collection_interval`
   - Disable expensive collectors
   - Limit monitored collections

### Configuration Debugging

```bash
# Enable debug logging
export LOG_LEVEL=debug
./mongo-exporter -config config.yaml

# Check configuration parsing
./mongo-exporter -config config.yaml --dump-config

# Validate specific sections
./mongo-exporter -config config.yaml --validate-mongodb
./mongo-exporter -config config.yaml --validate-metrics
``` 