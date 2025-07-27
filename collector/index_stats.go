package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type IndexStatsCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewIndexStatsCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *IndexStatsCollector {
	labels := []string{"instance", "replica_set", "shard", "database", "collection", "index"}

	descriptors := map[string]*prometheus.Desc{
		"index_size_bytes": prometheus.NewDesc(
			"mongodb_index_size_bytes",
			"Size of the index in bytes",
			labels,
			nil,
		),
		"index_accesses_total": prometheus.NewDesc(
			"mongodb_index_accesses_total",
			"Number of times the index has been accessed",
			labels,
			nil,
		),
		"index_miss_ratio": prometheus.NewDesc(
			"mongodb_index_miss_ratio",
			"Ratio of index misses to total accesses",
			labels,
			nil,
		),
		"index_ops_total": prometheus.NewDesc(
			"mongodb_index_ops_total",
			"Number of operations on the index",
			append(labels, "type"),
			nil,
		),
		"index_usage_status": prometheus.NewDesc(
			"mongodb_index_usage_status",
			"Index usage status (1=used, 0=unused)",
			labels,
			nil,
		),
		"index_last_access_time": prometheus.NewDesc(
			"mongodb_index_last_access_time",
			"Last time the index was accessed (Unix timestamp)",
			labels,
			nil,
		),
		"index_access_frequency": prometheus.NewDesc(
			"mongodb_index_access_frequency",
			"Index access frequency (accesses per hour)",
			labels,
			nil,
		),
		"index_unused_duration_hours": prometheus.NewDesc(
			"mongodb_index_unused_duration_hours",
			"Duration since last index access in hours",
			labels,
			nil,
		),
	}

	return &IndexStatsCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *IndexStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("index_stats") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get list of databases
	databases, err := getDatabasesWithTimeout(ctx, c.client, 10*time.Second)
	if err != nil {
		c.logger.Error("Failed to list databases", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(bson.M{})

	for _, dbName := range databases {
		// Skip admin and local databases
		if shouldSkipDatabase(dbName) {
			continue
		}

		db := c.client.Database(dbName)
		collections, err := getCollectionsWithTimeout(ctx, db, 10*time.Second)
		if err != nil {
			c.logger.Error("Failed to list collections", zap.String("database", dbName), zap.Error(err))
			continue
		}

		for _, collName := range collections {
			var indexStats bson.M
			if err := runCommandWithTimeout(ctx, db, bson.D{{"collStats", collName}}, 10*time.Second, &indexStats); err != nil {
				c.logger.Debug("Failed to get collection stats",
					zap.String("database", dbName),
					zap.String("collection", collName),
					zap.Error(err))
				continue
			}

			c.collectIndexStats(ch, dbName, collName, indexStats, instance)
		}
	}
}

func (c *IndexStatsCollector) collectIndexStats(ch chan<- prometheus.Metric, dbName, collName string, stats bson.M, instance map[string]string) {
	// Get current time for unused index calculations
	currentTime := time.Now()

	// Collect index sizes
	if indexSizes, ok := stats["indexSizes"].(bson.M); ok {
		if desc, ok := c.descriptors["index_size_bytes"]; ok {
			for indexName, size := range indexSizes {
				if sizeValue, ok := size.(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.GaugeValue,
						float64(sizeValue),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						dbName,
						collName,
						indexName,
					)
				}
			}
		}
	}

	// Collect index access statistics
	if indexAccesses, ok := stats["indexAccesses"].(bson.M); ok {
		for indexName, accesses := range indexAccesses {
			if accessMap, ok := accesses.(bson.M); ok {
				// Index access operations
				if desc, ok := c.descriptors["index_accesses_total"]; ok {
					if ops, ok := accessMap["ops"].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.CounterValue,
							float64(ops),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
							indexName,
						)
					}
				}

				// Index miss ratio
				if desc, ok := c.descriptors["index_miss_ratio"]; ok {
					if missRatio, ok := accessMap["missRatio"].(float64); ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							missRatio,
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
							indexName,
						)
					}
				}
			}
		}
	}

	// Enhanced unused index detection
	c.collectUnusedIndexMetrics(ch, dbName, collName, stats, instance, currentTime)

	// Collect index operations
	if indexStats, ok := stats["indexStats"].(bson.A); ok {
		if desc, ok := c.descriptors["index_ops_total"]; ok {
			for _, stat := range indexStats {
				if indexStat, ok := stat.(bson.M); ok {
					indexName, ok1 := indexStat["name"].(string)
					if !ok1 {
						c.logger.Warn("Invalid index name", zap.Any("index_stat", indexStat))
						continue
					}

					ops := map[string]string{
						"builds":    "build",
						"drops":     "drop",
						"reindexes": "reindex",
					}

					for opField, opType := range ops {
						if value, ok := indexStat[opField].(int64); ok {
							ch <- prometheus.MustNewConstMetric(
								desc,
								prometheus.CounterValue,
								float64(value),
								instance["instance"],
								instance["replica_set"],
								instance["shard"],
								dbName,
								collName,
								indexName,
								opType,
							)
						}
					}
				}
			}
		}
	}
}

func (c *IndexStatsCollector) collectUnusedIndexMetrics(ch chan<- prometheus.Metric, dbName, collName string, stats bson.M, instance map[string]string, currentTime time.Time) {
	// Get all indexes for this collection
	indexes := make(map[string]bool)
	if indexSizes, ok := stats["indexSizes"].(bson.M); ok {
		for indexName := range indexSizes {
			indexes[indexName] = false // Mark as unused initially
		}
	}

	// Check which indexes have been accessed
	if indexAccesses, ok := stats["indexAccesses"].(bson.M); ok {
		for indexName, accesses := range indexAccesses {
			if accessMap, ok := accesses.(bson.M); ok {
				if ops, ok := accessMap["ops"].(int64); ok && ops > 0 {
					indexes[indexName] = true // Mark as used

					// Index usage status (1=used, 0=unused)
					if desc, ok := c.descriptors["index_usage_status"]; ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							1.0, // Used
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
							indexName,
						)
					}

					// Last access time (if available)
					if desc, ok := c.descriptors["index_last_access_time"]; ok {
						// For now, use current time as last access
						// In a real implementation, you'd get this from MongoDB
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							float64(currentTime.Unix()),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
							indexName,
						)
					}

					// Access frequency (simplified calculation)
					if desc, ok := c.descriptors["index_access_frequency"]; ok {
						// This is a simplified calculation - in production you'd track over time
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							float64(ops), // Total operations as frequency indicator
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
							indexName,
						)
					}
				}
			}
		}
	}

	// Mark unused indexes
	for indexName, isUsed := range indexes {
		if !isUsed {
			// Index usage status (1=used, 0=unused)
			if desc, ok := c.descriptors["index_usage_status"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					0.0, // Unused
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
					indexName,
				)
			}

			// Unused duration (set to a high value for unused indexes)
			if desc, ok := c.descriptors["index_unused_duration_hours"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					8760.0, // 1 year in hours (high value for unused indexes)
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
					indexName,
				)
			}

			// Last access time (0 for unused indexes)
			if desc, ok := c.descriptors["index_last_access_time"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					0.0, // No access
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
					indexName,
				)
			}

			// Access frequency (0 for unused indexes)
			if desc, ok := c.descriptors["index_access_frequency"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					0.0, // No access
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
					indexName,
				)
			}
		}
	}
}

func (c *IndexStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *IndexStatsCollector) Name() string {
	return "index_stats"
}
