package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type OperationMetricsCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewOperationMetricsCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *OperationMetricsCollector {
	labels := []string{"instance", "replica_set", "shard"}

	descriptors := map[string]*prometheus.Desc{
		"metrics_operation_total":                 prometheus.NewDesc("mongodb_metrics_operation_total", "General operation metrics", labels, nil),
		"metrics_operation_fastmod_total":         prometheus.NewDesc("mongodb_metrics_operation_fastmod_total", "Total number of fast modify operations", labels, nil),
		"metrics_operation_idhack_total":          prometheus.NewDesc("mongodb_metrics_operation_idhack_total", "Total number of ID hack operations", labels, nil),
		"metrics_operation_scan_and_order_total":  prometheus.NewDesc("mongodb_metrics_operation_scan_and_order_total", "Total number of scan and order operations", labels, nil),
		"metrics_operation_write_conflicts_total": prometheus.NewDesc("mongodb_metrics_operation_write_conflicts_total", "Total number of write conflicts", labels, nil),
		"metrics_operation_commits_total":         prometheus.NewDesc("mongodb_metrics_operation_commits_total", "Total number of commits", labels, nil),
		"metrics_operation_rollbacks_total":       prometheus.NewDesc("mongodb_metrics_operation_rollbacks_total", "Total number of rollbacks", labels, nil),
		"metrics_operation_apply_ops_total":       prometheus.NewDesc("mongodb_metrics_operation_apply_ops_total", "Total number of apply operations", labels, nil),
		"metrics_operation_commands_total":        prometheus.NewDesc("mongodb_metrics_operation_commands_total", "Total number of commands", labels, nil),
	}

	return &OperationMetricsCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *OperationMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("operation_metrics") {
		return
	}

	ctx := context.Background()
	var result bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result)
	if err != nil {
		c.logger.Error("Failed to get server status for operation metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)
	c.collectOperationMetrics(ch, result, instance)
}

func (c *OperationMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *OperationMetricsCollector) Name() string {
	return "operation_metrics"
}

func (c *OperationMetricsCollector) getInstanceInfo(result bson.M) prometheus.Labels {
	labels := prometheus.Labels{
		"instance":    "unknown",
		"replica_set": "unknown",
		"shard":       "unknown",
	}

	if host, ok := result["host"].(string); ok {
		labels["instance"] = host
	}

	if repl, ok := result["repl"].(bson.M); ok {
		if setName, ok := repl["setName"].(string); ok {
			labels["replica_set"] = setName
		}
	}

	if shard, ok := result["shard"].(string); ok {
		labels["shard"] = shard
	}

	c.addCustomLabels(labels)
	return labels
}

func (c *OperationMetricsCollector) collectOperationMetrics(ch chan<- prometheus.Metric, result bson.M, labels prometheus.Labels) {
	if metrics, ok := result["metrics"].(bson.M); ok {
		if operation, ok := metrics["operation"].(bson.M); ok {
			c.collectOperationCounters(ch, operation, labels)
		}
	}
}

func (c *OperationMetricsCollector) collectOperationCounters(ch chan<- prometheus.Metric, operation bson.M, labels prometheus.Labels) {
	operationMetrics := map[string]string{
		"fastmod":        "metrics_operation_fastmod_total",
		"idhack":         "metrics_operation_idhack_total",
		"scanAndOrder":   "metrics_operation_scan_and_order_total",
		"writeConflicts": "metrics_operation_write_conflicts_total",
		"commits":        "metrics_operation_commits_total",
		"rollbacks":      "metrics_operation_rollbacks_total",
		"applyOps":       "metrics_operation_apply_ops_total",
		"commands":       "metrics_operation_commands_total",
	}

	for key, metricName := range operationMetrics {
		if value, ok := operation[key].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors[metricName], prometheus.CounterValue, float64(value), labels["instance"], labels["replica_set"], labels["shard"])
		}
	}
}
