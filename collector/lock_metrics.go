package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type LockMetricsCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewLockMetricsCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *LockMetricsCollector {
	labels := []string{"instance", "replica_set", "shard"}

	descriptors := map[string]*prometheus.Desc{
		"locks_time_acquiring_global_microseconds_total":     prometheus.NewDesc("mongodb_locks_time_acquiring_global_microseconds_total", "Total time spent acquiring global locks in microseconds", labels, nil),
		"locks_time_acquiring_database_microseconds_total":   prometheus.NewDesc("mongodb_locks_time_acquiring_database_microseconds_total", "Total time spent acquiring database locks in microseconds", labels, nil),
		"locks_time_acquiring_collection_microseconds_total": prometheus.NewDesc("mongodb_locks_time_acquiring_collection_microseconds_total", "Total time spent acquiring collection locks in microseconds", labels, nil),
		"locks_deadlock_count_total":                         prometheus.NewDesc("mongodb_locks_deadlock_count_total", "Total number of deadlocks", labels, nil),
		"locks_acquire_count_total":                          prometheus.NewDesc("mongodb_locks_acquire_count_total", "Total number of lock acquisitions", labels, nil),
		"locks_acquire_wait_count_total":                     prometheus.NewDesc("mongodb_locks_acquire_wait_count_total", "Total number of lock acquisitions that had to wait", labels, nil),
	}

	return &LockMetricsCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *LockMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("lock_metrics") {
		return
	}

	ctx := context.Background()
	var result bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result)
	if err != nil {
		c.logger.Error("Failed to get server status for lock metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)
	c.collectLockMetrics(ch, result, instance)
}

func (c *LockMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *LockMetricsCollector) Name() string {
	return "lock_metrics"
}

func (c *LockMetricsCollector) getInstanceInfo(result bson.M) prometheus.Labels {
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

func (c *LockMetricsCollector) collectLockMetrics(ch chan<- prometheus.Metric, result bson.M, labels prometheus.Labels) {
	if locks, ok := result["locks"].(bson.M); ok {
		c.collectGlobalLockMetrics(ch, locks, labels)
		c.collectDatabaseLockMetrics(ch, locks, labels)
		c.collectCollectionLockMetrics(ch, locks, labels)
	}
}

func (c *LockMetricsCollector) collectGlobalLockMetrics(ch chan<- prometheus.Metric, locks bson.M, labels prometheus.Labels) {
	if global, ok := locks["Global"].(bson.M); ok {
		if acquireCount, ok := global["acquireCount"].(bson.M); ok {
			if r, ok := acquireCount["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_acquire_count_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}

		if acquireWaitCount, ok := global["acquireWaitCount"].(bson.M); ok {
			if r, ok := acquireWaitCount["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_acquire_wait_count_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}

		if timeAcquiringMicros, ok := global["timeAcquiringMicros"].(bson.M); ok {
			if r, ok := timeAcquiringMicros["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_time_acquiring_global_microseconds_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}

		if deadlockCount, ok := global["deadlockCount"].(bson.M); ok {
			if r, ok := deadlockCount["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_deadlock_count_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}
	}
}

func (c *LockMetricsCollector) collectDatabaseLockMetrics(ch chan<- prometheus.Metric, locks bson.M, labels prometheus.Labels) {
	if database, ok := locks["Database"].(bson.M); ok {
		if timeAcquiringMicros, ok := database["timeAcquiringMicros"].(bson.M); ok {
			if r, ok := timeAcquiringMicros["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_time_acquiring_database_microseconds_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
			if w, ok := timeAcquiringMicros["w"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_time_acquiring_database_microseconds_total"], prometheus.CounterValue, float64(w), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}
	}
}

func (c *LockMetricsCollector) collectCollectionLockMetrics(ch chan<- prometheus.Metric, locks bson.M, labels prometheus.Labels) {
	if collection, ok := locks["Collection"].(bson.M); ok {
		if timeAcquiringMicros, ok := collection["timeAcquiringMicros"].(bson.M); ok {
			if r, ok := timeAcquiringMicros["r"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_time_acquiring_collection_microseconds_total"], prometheus.CounterValue, float64(r), labels["instance"], labels["replica_set"], labels["shard"])
			}
			if w, ok := timeAcquiringMicros["w"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(c.descriptors["locks_time_acquiring_collection_microseconds_total"], prometheus.CounterValue, float64(w), labels["instance"], labels["replica_set"], labels["shard"])
			}
		}
	}
}
 