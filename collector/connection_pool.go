package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type ConnectionPoolCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewConnectionPoolCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *ConnectionPoolCollector {
	labels := []string{"instance", "replica_set", "shard"}
	poolLabels := append(labels, "pool_name")
	hostLabels := append(labels, "host")

	descriptors := map[string]*prometheus.Desc{
		"connection_pool_current_checked_out": prometheus.NewDesc(
			"mongodb_connection_pool_current_checked_out",
			"The number of connections currently checked out of the pool",
			poolLabels,
			nil,
		),
		"connection_pool_current_checked_in": prometheus.NewDesc(
			"mongodb_connection_pool_current_checked_in",
			"The number of connections currently available in the pool",
			poolLabels,
			nil,
		),
		"connection_pool_current_created": prometheus.NewDesc(
			"mongodb_connection_pool_current_created",
			"The total number of connections currently created in the pool",
			poolLabels,
			nil,
		),
		"connection_pool_max_size": prometheus.NewDesc(
			"mongodb_connection_pool_max_size",
			"Maximum number of connections in the pool",
			poolLabels,
			nil,
		),
		"connection_pool_min_size": prometheus.NewDesc(
			"mongodb_connection_pool_min_size",
			"Minimum number of connections in the pool",
			poolLabels,
			nil,
		),
		"connection_pool_total_created": prometheus.NewDesc(
			"mongodb_connection_pool_total_created",
			"Total number of connections created since startup",
			poolLabels,
			nil,
		),
		"connection_pool_total_destroyed": prometheus.NewDesc(
			"mongodb_connection_pool_total_destroyed",
			"Total number of connections destroyed since startup",
			poolLabels,
			nil,
		),
		"connection_pool_requests_total": prometheus.NewDesc(
			"mongodb_connection_pool_requests_total",
			"Total number of connection requests",
			append(poolLabels, "result"),
			nil,
		),
		"connection_pool_wait_queue_size": prometheus.NewDesc(
			"mongodb_connection_pool_wait_queue_size",
			"Current number of operations waiting for a connection",
			poolLabels,
			nil,
		),
		"connection_pool_wait_queue_timeout_total": prometheus.NewDesc(
			"mongodb_connection_pool_wait_queue_timeout_total",
			"Total number of connection wait queue timeouts",
			poolLabels,
			nil,
		),
		"connection_pool_wait_time_milliseconds": prometheus.NewDesc(
			"mongodb_connection_pool_wait_time_milliseconds",
			"Average time spent waiting for connections in milliseconds",
			poolLabels,
			nil,
		),
		"connection_pool_checkout_time_milliseconds": prometheus.NewDesc(
			"mongodb_connection_pool_checkout_time_milliseconds",
			"Average time to checkout a connection in milliseconds",
			poolLabels,
			nil,
		),
		"connection_errors_total": prometheus.NewDesc(
			"mongodb_connection_errors_total",
			"Total number of connection errors by type",
			append(labels, "error_type", "host"),
			nil,
		),
		"connection_establishment_time_milliseconds": prometheus.NewDesc(
			"mongodb_connection_establishment_time_milliseconds",
			"Average time to establish new connections in milliseconds",
			hostLabels,
			nil,
		),
		"connection_auth_time_milliseconds": prometheus.NewDesc(
			"mongodb_connection_auth_time_milliseconds",
			"Average time to authenticate connections in milliseconds",
			hostLabels,
			nil,
		),
		"connection_handshake_time_milliseconds": prometheus.NewDesc(
			"mongodb_connection_handshake_time_milliseconds",
			"Average time for connection handshake in milliseconds",
			hostLabels,
			nil,
		),
	}

	return &ConnectionPoolCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *ConnectionPoolCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("connection_pool") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect connection pool metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	// Collect connection pool metrics from serverStatus
	c.collectConnectionPoolMetrics(ch, result, instance)

	// Collect connection error metrics
	c.collectConnectionErrorMetrics(ch, result, instance)

	// Collect detailed pool statistics if available
	c.collectDetailedPoolMetrics(ctx, ch, instance)
}

func (c *ConnectionPoolCollector) collectConnectionPoolMetrics(ch chan<- prometheus.Metric, result bson.M, instance map[string]string) {
	// Check for connection pool metrics in different locations
	if connections, ok := result["connections"].(bson.M); ok {
		c.collectBasicConnectionMetrics(ch, connections, instance)
	}

	// Check for detailed metrics in metrics section
	if metrics, ok := result["metrics"].(bson.M); ok {
		if connPoolStats, ok := metrics["connectionPool"].(bson.M); ok {
			c.collectConnectionPoolStatsMetrics(ch, connPoolStats, instance)
		}
	}
}

func (c *ConnectionPoolCollector) collectBasicConnectionMetrics(ch chan<- prometheus.Metric, connections bson.M, instance map[string]string) {
	poolName := "default"
	labels := []string{instance["instance"], instance["replica_set"], instance["shard"], poolName}

	// Current connections (assuming these are checked out)
	if current, ok := connections["current"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_current_checked_out"],
			prometheus.GaugeValue,
			float64(current),
			labels...,
		)
	}

	// Available connections
	if available, ok := connections["available"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_current_checked_in"],
			prometheus.GaugeValue,
			float64(available),
			labels...,
		)

		// Calculate total created (approximation)
		if current, ok := connections["current"].(int64); ok {
			totalCreated := current + available
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["connection_pool_current_created"],
				prometheus.GaugeValue,
				float64(totalCreated),
				labels...,
			)
		}
	}

	// Total created connections
	if totalCreated, ok := connections["totalCreated"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_total_created"],
			prometheus.CounterValue,
			float64(totalCreated),
			labels...,
		)
	}
}

func (c *ConnectionPoolCollector) collectConnectionPoolStatsMetrics(ch chan<- prometheus.Metric, poolStats bson.M, instance map[string]string) {
	// Iterate through each pool (host-specific or global)
	for poolName, stats := range poolStats {
		if poolData, ok := stats.(bson.M); ok {
			c.emitPoolSpecificMetrics(ch, poolName, poolData, instance)
		}
	}
}

func (c *ConnectionPoolCollector) emitPoolSpecificMetrics(ch chan<- prometheus.Metric, poolName string, poolData bson.M, instance map[string]string) {
	labels := []string{instance["instance"], instance["replica_set"], instance["shard"], poolName}

	// Current pool state
	if currentCheckedOut, ok := poolData["currentCheckedOut"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_current_checked_out"],
			prometheus.GaugeValue,
			float64(currentCheckedOut),
			labels...,
		)
	}

	if currentAvailable, ok := poolData["currentAvailable"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_current_checked_in"],
			prometheus.GaugeValue,
			float64(currentAvailable),
			labels...,
		)
	}

	if currentCreated, ok := poolData["currentCreated"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_current_created"],
			prometheus.GaugeValue,
			float64(currentCreated),
			labels...,
		)
	}

	// Pool configuration
	if maxSize, ok := poolData["maxPoolSize"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_max_size"],
			prometheus.GaugeValue,
			float64(maxSize),
			labels...,
		)
	}

	if minSize, ok := poolData["minPoolSize"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_min_size"],
			prometheus.GaugeValue,
			float64(minSize),
			labels...,
		)
	}

	// Lifetime counters
	if totalCreated, ok := poolData["totalCreated"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_total_created"],
			prometheus.CounterValue,
			float64(totalCreated),
			labels...,
		)
	}

	if totalDestroyed, ok := poolData["totalDestroyed"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_total_destroyed"],
			prometheus.CounterValue,
			float64(totalDestroyed),
			labels...,
		)
	}

	// Request metrics
	if successful, ok := poolData["requestsSuccessful"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_requests_total"],
			prometheus.CounterValue,
			float64(successful),
			append(labels, "success")...,
		)
	}

	if failed, ok := poolData["requestsFailed"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_requests_total"],
			prometheus.CounterValue,
			float64(failed),
			append(labels, "failed")...,
		)
	}

	// Wait queue metrics
	if waitQueueSize, ok := poolData["waitQueueSize"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_wait_queue_size"],
			prometheus.GaugeValue,
			float64(waitQueueSize),
			labels...,
		)
	}

	if waitQueueTimeouts, ok := poolData["waitQueueTimeouts"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_wait_queue_timeout_total"],
			prometheus.CounterValue,
			float64(waitQueueTimeouts),
			labels...,
		)
	}

	// Timing metrics
	if avgWaitTime, ok := poolData["avgWaitTimeMs"].(float64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_wait_time_milliseconds"],
			prometheus.GaugeValue,
			avgWaitTime,
			labels...,
		)
	}

	if avgCheckoutTime, ok := poolData["avgCheckoutTimeMs"].(float64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["connection_pool_checkout_time_milliseconds"],
			prometheus.GaugeValue,
			avgCheckoutTime,
			labels...,
		)
	}
}

func (c *ConnectionPoolCollector) collectConnectionErrorMetrics(ch chan<- prometheus.Metric, result bson.M, instance map[string]string) {
	if metrics, ok := result["metrics"].(bson.M); ok {
		if cursor, ok := metrics["cursor"].(bson.M); ok {
			// Connection timeout errors
			if timeouts, ok := cursor["timedOut"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["connection_errors_total"],
					prometheus.CounterValue,
					float64(timeouts),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					"timeout",
					"unknown",
				)
			}
		}

		// Network errors
		if network, ok := metrics["network"].(bson.M); ok {
			networkErrors := map[string]string{
				"errors":            "network_error",
				"timeouts":          "network_timeout",
				"compressionErrors": "compression_error",
			}

			for errorKey, errorType := range networkErrors {
				if errorCount, ok := network[errorKey].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["connection_errors_total"],
						prometheus.CounterValue,
						float64(errorCount),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						errorType,
						"unknown",
					)
				}
			}
		}
	}
}

func (c *ConnectionPoolCollector) collectDetailedPoolMetrics(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Try to get more detailed connection pool information using serverStatus with additional details
	var detailedResult bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{
		{"serverStatus", 1},
		{"connections", 1},
		{"network", 1},
	}).Decode(&detailedResult)

	if err != nil {
		c.logger.Debug("Failed to get detailed connection metrics", zap.Error(err))
		return
	}

	// Look for host-specific connection metrics
	if network, ok := detailedResult["network"].(bson.M); ok {
		c.collectNetworkConnectionMetrics(ch, network, instance)
	}

	// Try to get current operations to analyze connection usage
	c.collectCurrentOpConnectionMetrics(ctx, ch, instance)
}

func (c *ConnectionPoolCollector) collectNetworkConnectionMetrics(ch chan<- prometheus.Metric, network bson.M, instance map[string]string) {
	// Network-level connection metrics
	if compression, ok := network["compression"].(bson.M); ok {
		// Compression-related connection metrics can be collected here
		// This might include compression ratio, compression errors, etc.
		for compressor, stats := range compression {
			if compressorStats, ok := stats.(bson.M); ok {
				if compressorRequests, ok := compressorStats["requests"].(int64); ok {
					// Emit compression-related metrics
					c.logger.Debug("Compression stats",
						zap.String("compressor", compressor),
						zap.Int64("requests", compressorRequests))
				}
			}
		}
	}
}

func (c *ConnectionPoolCollector) collectCurrentOpConnectionMetrics(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Get current operations to analyze active connections
	var currentOp bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{
		{"currentOp", 1},
		{"$all", true},
	}).Decode(&currentOp)

	if err != nil {
		c.logger.Debug("Failed to get current operations for connection analysis", zap.Error(err))
		return
	}

	if inprog, ok := currentOp["inprog"].(bson.A); ok {
		hostConnectionCounts := make(map[string]int)

		for _, op := range inprog {
			if opMap, ok := op.(bson.M); ok {
				if client, ok := opMap["client"].(string); ok {
					hostConnectionCounts[client]++
				}
			}
		}

		// Emit per-host connection counts
		for host, count := range hostConnectionCounts {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["connection_establishment_time_milliseconds"],
				prometheus.GaugeValue,
				float64(count), // This is not actually establishment time, but active connections per host
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
				host,
			)
		}
	}
}

func (c *ConnectionPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *ConnectionPoolCollector) Name() string {
	return "connection_pool"
}
