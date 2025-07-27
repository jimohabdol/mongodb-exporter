package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type ProfileCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
	lastCheck   time.Time
}

func NewProfileCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *ProfileCollector {
	labels := []string{"instance", "replica_set", "shard", "database"}
	operationLabels := append(labels, "operation", "collection")
	planSummaryLabels := append(labels, "plan_summary")

	descriptors := map[string]*prometheus.Desc{
		"profile_slow_operations_total": prometheus.NewDesc(
			"mongodb_profile_slow_operations_total",
			"Total number of slow operations by type",
			operationLabels,
			nil,
		),
		"profile_operations_duration_seconds": prometheus.NewDesc(
			"mongodb_profile_operations_duration_seconds",
			"Duration histogram of profiled operations in seconds",
			operationLabels,
			nil,
		),
		"profile_operations_examined_docs": prometheus.NewDesc(
			"mongodb_profile_operations_examined_docs",
			"Number of documents examined by profiled operations",
			operationLabels,
			nil,
		),
		"profile_operations_docs_returned": prometheus.NewDesc(
			"mongodb_profile_operations_docs_returned",
			"Number of documents returned by profiled operations",
			operationLabels,
			nil,
		),
		"profile_operations_keys_examined": prometheus.NewDesc(
			"mongodb_profile_operations_keys_examined",
			"Number of index keys examined by profiled operations",
			operationLabels,
			nil,
		),
		"profile_operations_response_length_bytes": prometheus.NewDesc(
			"mongodb_profile_operations_response_length_bytes",
			"Response length in bytes for profiled operations",
			operationLabels,
			nil,
		),
		"profile_operations_locks_acquired": prometheus.NewDesc(
			"mongodb_profile_operations_locks_acquired",
			"Number of locks acquired during profiled operations",
			append(operationLabels, "lock_type"),
			nil,
		),
		"profile_operations_lock_wait_time_microseconds": prometheus.NewDesc(
			"mongodb_profile_operations_lock_wait_time_microseconds",
			"Time spent waiting for locks during profiled operations in microseconds",
			append(operationLabels, "lock_type"),
			nil,
		),
		"profile_plan_summary_total": prometheus.NewDesc(
			"mongodb_profile_plan_summary_total",
			"Total number of operations by execution plan summary",
			planSummaryLabels,
			nil,
		),
		"profile_write_conflicts_total": prometheus.NewDesc(
			"mongodb_profile_write_conflicts_total",
			"Total number of write conflicts in profiled operations",
			operationLabels,
			nil,
		),
		"profile_storage_stats_total": prometheus.NewDesc(
			"mongodb_profile_storage_stats_total",
			"Storage engine statistics from profiled operations",
			append(operationLabels, "storage_stat"),
			nil,
		),
		"profile_cpu_time_microseconds": prometheus.NewDesc(
			"mongodb_profile_cpu_time_microseconds",
			"CPU time used by profiled operations in microseconds",
			operationLabels,
			nil,
		),
	}

	return &ProfileCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
		lastCheck:     time.Now().Add(-1 * time.Hour), // Start 1 hour ago
	}
}

func (c *ProfileCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("profile") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get list of databases
	databases, err := c.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		c.logger.Error("Failed to list databases for profiling", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(bson.M{})

	currentTime := time.Now()

	for _, dbName := range databases {
		// Skip system databases unless explicitly requested
		if c.shouldSkipDatabase(dbName) {
			continue
		}

		c.collectDatabaseProfileMetrics(ctx, ch, dbName, instance, c.lastCheck, currentTime)
	}

	c.lastCheck = currentTime
}

func (c *ProfileCollector) collectDatabaseProfileMetrics(ctx context.Context, ch chan<- prometheus.Metric, dbName string, instance map[string]string, since, until time.Time) {
	db := c.client.Database(dbName)

	// Check if profiling is enabled
	var profileStatus bson.M
	err := db.RunCommand(ctx, bson.D{{"profile", -1}}).Decode(&profileStatus)
	if err != nil {
		c.logger.Debug("Failed to get profile status",
			zap.String("database", dbName),
			zap.Error(err))
		return
	}

	// Skip if profiling is disabled
	if level, ok := profileStatus["was"].(int32); ok && level == 0 {
		return
	}

	// Query the profile collection for operations since last check
	collection := db.Collection("system.profile")

	filter := bson.D{
		{"ts", bson.D{
			{"$gte", primitive.NewDateTimeFromTime(since)},
			{"$lt", primitive.NewDateTimeFromTime(until)},
		}},
	}

	cursor, err := collection.Find(ctx, filter, options.Find().SetSort(bson.D{{"ts", -1}}))
	if err != nil {
		c.logger.Debug("Failed to query profile collection",
			zap.String("database", dbName),
			zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	// Process profile entries
	var profileEntries []bson.M
	if err := cursor.All(ctx, &profileEntries); err != nil {
		c.logger.Error("Failed to decode profile entries",
			zap.String("database", dbName),
			zap.Error(err))
		return
	}

	// Aggregate metrics from profile entries
	c.aggregateProfileMetrics(ch, profileEntries, dbName, instance)
}

func (c *ProfileCollector) aggregateProfileMetrics(ch chan<- prometheus.Metric, entries []bson.M, dbName string, instance map[string]string) {
	operationStats := make(map[string]*OperationStats)
	planSummaryStats := make(map[string]int64)

	for _, entry := range entries {
		op := c.extractOperationType(entry)
		collection := c.extractCollection(entry)
		key := op + ":" + collection

		if _, exists := operationStats[key]; !exists {
			operationStats[key] = &OperationStats{
				Operation:  op,
				Collection: collection,
			}
		}

		stats := operationStats[key]
		stats.Count++

		// Duration
		if millis, ok := entry["millis"].(int64); ok {
			stats.TotalDurationMs += millis
			if millis > stats.MaxDurationMs {
				stats.MaxDurationMs = millis
			}
		}

		// Execution stats
		if execStats, ok := entry["execStats"].(bson.M); ok {
			if examined, ok := execStats["totalDocsExamined"].(int64); ok {
				stats.TotalDocsExamined += examined
			}
			if returned, ok := execStats["totalDocsReturned"].(int64); ok {
				stats.TotalDocsReturned += returned
			}
			if keysExamined, ok := execStats["totalKeysExamined"].(int64); ok {
				stats.TotalKeysExamined += keysExamined
			}
		}

		// Response length
		if responseLength, ok := entry["responseLength"].(int64); ok {
			stats.TotalResponseLength += responseLength
		}

		// Plan summary
		if planSummary, ok := entry["planSummary"].(string); ok {
			planSummaryStats[planSummary]++
		}

		// Lock statistics
		c.collectLockStats(entry, stats)

		// Write conflicts
		if writeConflicts, ok := entry["writeConflicts"].(int64); ok {
			stats.WriteConflicts += writeConflicts
		}

		// Storage stats
		c.collectStorageStats(entry, stats)

		// CPU time (if available)
		if cpuTime, ok := entry["cpuNanos"].(int64); ok {
			stats.CpuTimeMicros += cpuTime / 1000 // Convert nanos to micros
		}
	}

	// Emit metrics
	c.emitOperationMetrics(ch, operationStats, dbName, instance)
	c.emitPlanSummaryMetrics(ch, planSummaryStats, dbName, instance)
}

func (c *ProfileCollector) emitOperationMetrics(ch chan<- prometheus.Metric, stats map[string]*OperationStats, dbName string, instance map[string]string) {
	for _, stat := range stats {
		labels := []string{
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
			dbName,
			stat.Operation,
			stat.Collection,
		}

		// Total operations
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["profile_slow_operations_total"],
			prometheus.CounterValue,
			float64(stat.Count),
			labels...,
		)

		if stat.Count > 0 {
			// Average duration
			avgDuration := float64(stat.TotalDurationMs) / float64(stat.Count) / 1000.0 // Convert to seconds
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["profile_operations_duration_seconds"],
				prometheus.GaugeValue,
				avgDuration,
				labels...,
			)

			// Documents examined
			if stat.TotalDocsExamined > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_operations_examined_docs"],
					prometheus.CounterValue,
					float64(stat.TotalDocsExamined),
					labels...,
				)
			}

			// Documents returned
			if stat.TotalDocsReturned > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_operations_docs_returned"],
					prometheus.CounterValue,
					float64(stat.TotalDocsReturned),
					labels...,
				)
			}

			// Keys examined
			if stat.TotalKeysExamined > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_operations_keys_examined"],
					prometheus.CounterValue,
					float64(stat.TotalKeysExamined),
					labels...,
				)
			}

			// Response length
			if stat.TotalResponseLength > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_operations_response_length_bytes"],
					prometheus.CounterValue,
					float64(stat.TotalResponseLength),
					labels...,
				)
			}

			// Write conflicts
			if stat.WriteConflicts > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_write_conflicts_total"],
					prometheus.CounterValue,
					float64(stat.WriteConflicts),
					labels...,
				)
			}

			// CPU time
			if stat.CpuTimeMicros > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_cpu_time_microseconds"],
					prometheus.CounterValue,
					float64(stat.CpuTimeMicros),
					labels...,
				)
			}

			// Lock metrics
			for lockType, lockStat := range stat.LockStats {
				lockLabels := append(labels, lockType)

				if lockStat.AcquireCount > 0 {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["profile_operations_locks_acquired"],
						prometheus.CounterValue,
						float64(lockStat.AcquireCount),
						lockLabels...,
					)
				}

				if lockStat.AcquireWaitCount > 0 {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["profile_operations_lock_wait_time_microseconds"],
						prometheus.CounterValue,
						float64(lockStat.TimeAcquiringMicros),
						lockLabels...,
					)
				}
			}

			// Storage stats
			for statName, value := range stat.StorageStats {
				storageLabels := append(labels, statName)
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["profile_storage_stats_total"],
					prometheus.CounterValue,
					float64(value),
					storageLabels...,
				)
			}
		}
	}
}

func (c *ProfileCollector) emitPlanSummaryMetrics(ch chan<- prometheus.Metric, planStats map[string]int64, dbName string, instance map[string]string) {
	for planSummary, count := range planStats {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["profile_plan_summary_total"],
			prometheus.CounterValue,
			float64(count),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
			dbName,
			planSummary,
		)
	}
}

func (c *ProfileCollector) extractOperationType(entry bson.M) string {
	if op, ok := entry["op"].(string); ok {
		return op
	}
	if command, ok := entry["command"].(bson.M); ok {
		// Try to extract command type from command object
		for cmdType := range command {
			if cmdType != "filter" && cmdType != "sort" && cmdType != "projection" {
				return cmdType
			}
		}
	}
	return "unknown"
}

func (c *ProfileCollector) extractCollection(entry bson.M) string {
	if ns, ok := entry["ns"].(string); ok {
		// Extract collection name from namespace (db.collection)
		for i := len(ns) - 1; i >= 0; i-- {
			if ns[i] == '.' {
				return ns[i+1:]
			}
		}
		return ns
	}
	return "unknown"
}

func (c *ProfileCollector) collectLockStats(entry bson.M, stats *OperationStats) {
	if locks, ok := entry["locks"].(bson.M); ok {
		if stats.LockStats == nil {
			stats.LockStats = make(map[string]*LockStat)
		}

		for lockType, lockData := range locks {
			if lockInfo, ok := lockData.(bson.M); ok {
				if stats.LockStats[lockType] == nil {
					stats.LockStats[lockType] = &LockStat{}
				}

				lockStat := stats.LockStats[lockType]

				if acquireCount, ok := lockInfo["acquireCount"].(bson.M); ok {
					for mode, count := range acquireCount {
						if c, ok := count.(int64); ok && mode == "r" || mode == "w" {
							lockStat.AcquireCount += c
						}
					}
				}

				if timeAcquiring, ok := lockInfo["timeAcquiringMicros"].(bson.M); ok {
					for mode, time := range timeAcquiring {
						if t, ok := time.(int64); ok && mode == "r" || mode == "w" {
							lockStat.TimeAcquiringMicros += t
						}
					}
				}

				if acquireWaitCount, ok := lockInfo["acquireWaitCount"].(bson.M); ok {
					for mode, count := range acquireWaitCount {
						if c, ok := count.(int64); ok && mode == "r" || mode == "w" {
							lockStat.AcquireWaitCount += c
						}
					}
				}
			}
		}
	}
}

func (c *ProfileCollector) collectStorageStats(entry bson.M, stats *OperationStats) {
	if storage, ok := entry["storage"].(bson.M); ok {
		if stats.StorageStats == nil {
			stats.StorageStats = make(map[string]int64)
		}

		// Common storage metrics
		storageMetrics := []string{
			"data_read", "data_written", "index_read", "index_written",
			"cache_hits", "cache_misses", "pages_read", "pages_written",
		}

		for _, metric := range storageMetrics {
			if value, ok := storage[metric].(int64); ok {
				stats.StorageStats[metric] += value
			}
		}
	}
}

func (c *ProfileCollector) shouldSkipDatabase(dbName string) bool {
	// Skip admin, config, and local databases unless explicitly requested
	systemDatabases := []string{"admin", "config", "local"}
	for _, sysDB := range systemDatabases {
		if dbName == sysDB {
			return true
		}
	}
	return false
}

type OperationStats struct {
	Operation           string
	Collection          string
	Count               int64
	TotalDurationMs     int64
	MaxDurationMs       int64
	TotalDocsExamined   int64
	TotalDocsReturned   int64
	TotalKeysExamined   int64
	TotalResponseLength int64
	WriteConflicts      int64
	CpuTimeMicros       int64
	LockStats           map[string]*LockStat
	StorageStats        map[string]int64
}

type LockStat struct {
	AcquireCount        int64
	AcquireWaitCount    int64
	TimeAcquiringMicros int64
}

func (c *ProfileCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *ProfileCollector) Name() string {
	return "profile"
}
 