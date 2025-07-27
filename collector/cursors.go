package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type CursorCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewCursorCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *CursorCollector {
	labels := []string{"instance", "replica_set", "shard"}
	cursorLabels := append(labels, "cursor_type")
	operationLabels := append(labels, "operation")

	descriptors := map[string]*prometheus.Desc{
		"cursors_open": prometheus.NewDesc(
			"mongodb_cursors_open",
			"Number of open cursors by type",
			cursorLabels,
			nil,
		),
		"cursors_timed_out_total": prometheus.NewDesc(
			"mongodb_cursors_timed_out_total",
			"Total number of cursors that have timed out since the server was started",
			labels,
			nil,
		),
		"cursor_timeout_seconds": prometheus.NewDesc(
			"mongodb_cursor_timeout_seconds",
			"Current cursor timeout value in seconds",
			labels,
			nil,
		),
		"cursors_killed_total": prometheus.NewDesc(
			"mongodb_cursors_killed_total",
			"Total number of cursors killed by operation",
			operationLabels,
			nil,
		),
		"cursors_created_total": prometheus.NewDesc(
			"mongodb_cursors_created_total",
			"Total number of cursors created since server start",
			labels,
			nil,
		),
		"cursor_pool_size": prometheus.NewDesc(
			"mongodb_cursor_pool_size",
			"Current size of the cursor pool",
			labels,
			nil,
		),
		"cursor_memory_usage_bytes": prometheus.NewDesc(
			"mongodb_cursor_memory_usage_bytes",
			"Total memory usage by open cursors in bytes",
			labels,
			nil,
		),
		"cursor_getmore_operations_total": prometheus.NewDesc(
			"mongodb_cursor_getmore_operations_total",
			"Total number of getMore operations performed",
			labels,
			nil,
		),
		"cursor_batch_size_avg": prometheus.NewDesc(
			"mongodb_cursor_batch_size_avg",
			"Average batch size of cursor operations",
			labels,
			nil,
		),
		"pinned_cursors": prometheus.NewDesc(
			"mongodb_pinned_cursors",
			"Number of pinned cursors",
			labels,
			nil,
		),
	}

	return &CursorCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *CursorCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("cursors") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect cursor metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	// Collect basic cursor metrics from serverStatus
	c.collectBasicCursorMetrics(ch, result, instance)

	// Collect additional cursor metrics from currentOp
	c.collectCurrentOpCursorMetrics(ctx, ch, instance)

	// Collect cursor kill statistics
	c.collectCursorKillMetrics(ctx, ch, result, instance)

	// Collect global cursor timeout settings
	c.collectCursorTimeoutSettings(ctx, ch, instance)
}

func (c *CursorCollector) collectBasicCursorMetrics(ch chan<- prometheus.Metric, result bson.M, instance map[string]string) {
	// Get metrics from serverStatus
	if metrics, ok := result["metrics"].(bson.M); ok {
		if cursor, ok := metrics["cursor"].(bson.M); ok {
			// Total cursors timed out
			if timedOut, ok := cursor["timedOut"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["cursors_timed_out_total"],
					prometheus.CounterValue,
					float64(timedOut),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}

			// Total cursors created
			if totalOpened, ok := cursor["totalOpened"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["cursors_created_total"],
					prometheus.CounterValue,
					float64(totalOpened),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}

			// Currently open cursors by type
			if open, ok := cursor["open"].(bson.M); ok {
				cursorTypes := map[string]string{
					"noTimeout": "no_timeout",
					"pinned":    "pinned",
					"total":     "total",
				}

				for serverKey, labelValue := range cursorTypes {
					if count, ok := open[serverKey].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							c.descriptors["cursors_open"],
							prometheus.GaugeValue,
							float64(count),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							labelValue,
						)
					}
				}

				// Pinned cursors (specific metric)
				if pinned, ok := open["pinned"].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["pinned_cursors"],
						prometheus.GaugeValue,
						float64(pinned),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
					)
				}
			}
		}

		// GetMore operations from operation metrics
		if operation, ok := metrics["operation"].(bson.M); ok {
			if getmore, ok := operation["getmore"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["cursor_getmore_operations_total"],
					prometheus.CounterValue,
					float64(getmore),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}
		}
	}
}

func (c *CursorCollector) collectCurrentOpCursorMetrics(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	var currentOp bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{
		{"currentOp", 1},
		{"$all", true},
	}).Decode(&currentOp)

	if err != nil {
		c.logger.Debug("Failed to run currentOp command for cursor metrics", zap.Error(err))
		return
	}
	if inprog, ok := currentOp["inprog"].(bson.A); ok {
		var totalMemoryUsage int64
		var activeCursorCount int
		var totalBatchSize int64
		var batchCount int

		for _, op := range inprog {
			if opMap, ok := op.(bson.M); ok {
				if cursorInfo, ok := opMap["cursor"].(bson.M); ok {
					activeCursorCount++

					if memUsage := c.getNumericValue(cursorInfo["memUsage"]); memUsage != nil {
						totalMemoryUsage += int64(*memUsage)
					}

					if batchSize := c.getNumericValue(cursorInfo["batchSize"]); batchSize != nil {
						totalBatchSize += int64(*batchSize)
						batchCount++
					}
				}
			}
		}

		if totalMemoryUsage > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["cursor_memory_usage_bytes"],
				prometheus.GaugeValue,
				float64(totalMemoryUsage),
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
			)
		}

		if batchCount > 0 {
			avgBatchSize := float64(totalBatchSize) / float64(batchCount)
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["cursor_batch_size_avg"],
				prometheus.GaugeValue,
				avgBatchSize,
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
			)
		}
	}
}

func (c *CursorCollector) collectCursorKillMetrics(ctx context.Context, ch chan<- prometheus.Metric, result bson.M, instance map[string]string) {
	if opcounters, ok := result["opcounters"].(bson.M); ok {
		if value := c.getNumericValue(opcounters["killcursors"]); value != nil {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["cursors_killed_total"],
				prometheus.CounterValue,
				*value,
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
				"killcursors_command",
			)
		}
	}

	if metrics, ok := result["metrics"].(bson.M); ok {
		if cursor, ok := metrics["cursor"].(bson.M); ok {
			if value := c.getNumericValue(cursor["totalKilled"]); value != nil {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["cursors_killed_total"],
					prometheus.CounterValue,
					*value,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					"timeout",
				)
			}
		}
	}
}

func (c *CursorCollector) collectCursorTimeoutSettings(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	var params bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"getParameter", 1}, {"cursorTimeoutMillis", 1}}).Decode(&params)
	if err != nil {
		c.logger.Debug("Failed to get cursor timeout parameters", zap.Error(err))
		err = c.client.Database("admin").RunCommand(ctx, bson.D{{"getParameter", 1}, {"clientCursorMonitorFrequencySecs", 1}}).Decode(&params)
		if err != nil {
			return
		}
	}

	if value := c.getNumericValue(params["cursorTimeoutMillis"]); value != nil {
		timeoutSeconds := *value / 1000.0
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["cursor_timeout_seconds"],
			prometheus.GaugeValue,
			timeoutSeconds,
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}

	if value := c.getNumericValue(params["clientCursorMonitorFrequencySecs"]); value != nil {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["cursor_timeout_seconds"],
			prometheus.GaugeValue,
			*value,
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}
}

func (c *CursorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *CursorCollector) Name() string {
	return "cursors"
}
 