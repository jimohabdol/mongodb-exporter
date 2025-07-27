package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type LockCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewLockCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *LockCollector {
	labels := []string{"instance", "replica_set", "shard", "database", "lock_type"}

	descriptors := map[string]*prometheus.Desc{
		"locks_time_acquiring_microseconds_total": prometheus.NewDesc(
			"mongodb_locks_time_acquiring_microseconds_total",
			"Time spent acquiring locks in microseconds",
			labels,
			nil,
		),
		"locks_held_total": prometheus.NewDesc(
			"mongodb_locks_held_total",
			"Number of locks held",
			labels,
			nil,
		),
		"locks_waiting_total": prometheus.NewDesc(
			"mongodb_locks_waiting_total",
			"Number of locks waiting to be acquired",
			labels,
			nil,
		),
		"locks_deadlock_total": prometheus.NewDesc(
			"mongodb_locks_deadlock_total",
			"Number of deadlocks",
			labels,
			nil,
		),
	}

	return &LockCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *LockCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("locks") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect lock metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	if locks, ok := result["locks"].(bson.M); ok {
		for dbName, dbLocks := range locks {
			if dbLocksMap, ok := dbLocks.(bson.M); ok {
				c.collectDatabaseLockMetrics(ch, dbName, dbLocksMap, instance)
			}
		}
	}
}

func (c *LockCollector) collectDatabaseLockMetrics(ch chan<- prometheus.Metric, dbName string, dbLocks bson.M, instance map[string]string) {
	lockTypes := []string{"ParallelBatchWriterMode", "ReplicationStateTransition", "Global", "Database", "Collection", "Mutex", "Metadata"}

	for _, lockType := range lockTypes {
		if lockMetrics, ok := dbLocks[lockType].(bson.M); ok {
			// Acquire time metrics
			if acquireTime, ok := lockMetrics["acquireCount"].(bson.M); ok {
				modes := map[string]string{"R": "read", "W": "write", "r": "intent_read", "w": "intent_write"}
				for mode, modeLabel := range modes {
					if count, ok := acquireTime[mode].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							c.descriptors["locks_time_acquiring_microseconds_total"],
							prometheus.CounterValue,
							float64(count),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							lockType+"_"+modeLabel,
						)
					}
				}
			}

			// Deadlock metrics
			if deadlocks, ok := lockMetrics["deadlockCount"].(bson.M); ok {
				modes := map[string]string{"R": "read", "W": "write", "r": "intent_read", "w": "intent_write"}
				for mode, modeLabel := range modes {
					if count, ok := deadlocks[mode].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							c.descriptors["locks_deadlock_total"],
							prometheus.CounterValue,
							float64(count),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							lockType+"_"+modeLabel,
						)
					}
				}
			}

			// Queue length metrics
			if queueLength, ok := lockMetrics["acquireWaitCount"].(bson.M); ok {
				modes := map[string]string{"R": "read", "W": "write", "r": "intent_read", "w": "intent_write"}
				for mode, modeLabel := range modes {
					if count, ok := queueLength[mode].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							c.descriptors["locks_waiting_total"],
							prometheus.GaugeValue,
							float64(count),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							lockType+"_"+modeLabel,
						)
					}
				}
			}
		}
	}
}

func (c *LockCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *LockCollector) Name() string {
	return "locks"
}
