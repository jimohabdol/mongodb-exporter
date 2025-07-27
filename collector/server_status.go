package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type ServerStatusCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewServerStatusCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *ServerStatusCollector {
	labels := []string{"instance", "replica_set", "shard"}

	descriptors := map[string]*prometheus.Desc{
		"uptime_seconds": prometheus.NewDesc(
			"mongodb_instance_uptime_seconds",
			"The uptime of the MongoDB instance in seconds",
			labels,
			nil,
		),
		"connections": prometheus.NewDesc(
			"mongodb_connections",
			"The current connections metrics",
			append(labels, "state"),
			nil,
		),
		"memory": prometheus.NewDesc(
			"mongodb_memory_bytes",
			"The current memory usage in bytes",
			append(labels, "type"),
			nil,
		),
		"extra_info": prometheus.NewDesc(
			"mongodb_extra_info",
			"Extra information metrics",
			append(labels, "type"),
			nil,
		),
		"network_bytes_total": prometheus.NewDesc(
			"mongodb_network_bytes_total",
			"Network traffic metrics",
			append(labels, "direction"),
			nil,
		),
		"op_counters_total": prometheus.NewDesc(
			"mongodb_op_counters_total",
			"Operation counters",
			append(labels, "type"),
			nil,
		),
		"metrics_document_total": prometheus.NewDesc(
			"mongodb_metrics_document_total",
			"Document operation metrics",
			append(labels, "type"),
			nil,
		),
		"connections_metrics": prometheus.NewDesc(
			"mongodb_connections_metrics",
			"Connections metrics",
			append(labels, "type"),
			nil,
		),
		"page_faults_total": prometheus.NewDesc(
			"mongodb_page_faults_total",
			"Page fault statistics",
			labels,
			nil,
		),
	}

	return &ServerStatusCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *ServerStatusCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("server_status") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result)
	if err != nil {
		c.logger.Error("Failed to get server status", zap.Error(err))
		return
	}

	c.collectMetrics(ctx, ch, result)
}

func (c *ServerStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *ServerStatusCollector) Name() string {
	return "server_status"
}

func (c *ServerStatusCollector) collectMetrics(ctx context.Context, ch chan<- prometheus.Metric, result bson.M) {
	instance := c.getInstanceInfo(result)

	// Uptime with validation
	if uptime, ok := result["uptime"].(float64); ok && uptime >= 0 {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["uptime_seconds"],
			prometheus.GaugeValue,
			uptime,
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}

	// Connections
	if connections, ok := result["connections"].(bson.M); ok {
		states := map[string]string{
			"current":      "current",
			"available":    "available",
			"active":       "active",
			"totalCreated": "total_created",
		}
		for stateKey, stateLabel := range states {
			if value := c.getNumericValue(connections[stateKey]); value != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["connections"],
					prometheus.GaugeValue,
					*value,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					stateLabel,
				)
			}
		}

		if metrics, ok := connections["metrics"].(bson.M); ok {
			metricTypes := map[string]string{
				"awaitingTopology": "awaiting_topology",
				"pending":          "pending",
				"rejected":         "rejected",
				"timedOut":         "timed_out",
			}
			for metricKey, metricLabel := range metricTypes {
				if value := c.getNumericValue(metrics[metricKey]); value != nil {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["connections_metrics"],
						prometheus.CounterValue,
						*value,
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						metricLabel,
					)
				}
			}
		}
	}

	if mem, ok := result["mem"].(bson.M); ok {
		memTypes := map[string]string{
			"resident":          "resident",
			"virtual":           "virtual",
			"mapped":            "mapped",
			"mappedWithJournal": "mapped_with_journal",
		}
		for memType, label := range memTypes {
			if value := c.getNumericValue(mem[memType]); value != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["memory"],
					prometheus.GaugeValue,
					*value*1024*1024,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					label,
				)
			}
		}
	}

	if extraInfo, ok := result["extra_info"].(bson.M); ok {
		if value := c.getNumericValue(extraInfo["page_faults"]); value != nil {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["page_faults_total"],
				prometheus.CounterValue,
				*value,
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
			)
		}

		metrics := map[string]string{
			"heap_usage_bytes":     "heap_usage",
			"page_faults":          "page_faults",
			"freeMonitoringStatus": "free_monitoring_status",
		}
		for metric, label := range metrics {
			if value := c.getNumericValue(extraInfo[metric]); value != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["extra_info"],
					prometheus.GaugeValue,
					*value,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					label,
				)
			}
		}
	}

	if network, ok := result["network"].(bson.M); ok {
		networkMetrics := map[string]string{
			"bytesIn":  "in",
			"bytesOut": "out",
		}
		for metricKey, direction := range networkMetrics {
			if value := c.getNumericValue(network[metricKey]); value != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["network_bytes_total"],
					prometheus.CounterValue,
					*value,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					direction,
				)
			}
		}
	}

	if opCounters, ok := result["opcounters"].(bson.M); ok {
		for opType, value := range opCounters {
			if val := c.getNumericValue(value); val != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["op_counters_total"],
					prometheus.CounterValue,
					*val,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					opType,
				)
			}
		}
	}

	if metrics, ok := result["metrics"].(bson.M); ok {
		if document, ok := metrics["document"].(bson.M); ok {
			types := []string{"deleted", "inserted", "returned", "updated"}
			for _, docType := range types {
				if value := c.getNumericValue(document[docType]); value != nil {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["metrics_document_total"],
						prometheus.CounterValue,
						*value,
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						docType,
					)
				}
			}
		}
	}
}
