package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type CompatibilityCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewCompatibilityCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *CompatibilityCollector {
	labels := []string{"instance", "replica_set", "shard"}
	opLabels := append(labels, "type")

	descriptors := map[string]*prometheus.Desc{
		// Only include metrics that aren't already provided by other collectors
		"op_counters_repl_total": prometheus.NewDesc(
			"mongodb_op_counters_repl_total",
			"Replication operation counters for dashboard 2583 compatibility",
			opLabels,
			nil,
		),
	}

	return &CompatibilityCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *CompatibilityCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("compatibility") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect compatibility metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	// Collect replication operation counters
	if opCountersRepl, ok := result["opcountersRepl"].(bson.M); ok {
		for opType, value := range opCountersRepl {
			if val, ok := value.(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["op_counters_repl_total"],
					prometheus.CounterValue,
					float64(val),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					opType,
				)
			}
		}
	}
}

func (c *CompatibilityCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *CompatibilityCollector) Name() string {
	return "compatibility"
}
