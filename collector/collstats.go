package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type CollStatsCollector struct {
	*BaseCollector
	descriptors          map[string]*prometheus.Desc
	monitoredCollections []string
}

func NewCollStatsCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *CollStatsCollector {
	labels := []string{"instance", "replica_set", "shard", "database", "collection"}
	indexLabels := append(labels, "index")

	// Load monitored collections from config
	var configMonitoredCollections []string
	if collStatsConfig, ok := config.Collectors["collstats"]; ok {
		if collStats, ok := collStatsConfig.(map[string]interface{}); ok {
			if monitored, ok := collStats["monitored_collections"].([]string); ok {
				configMonitoredCollections = monitored
			} else if monitored, ok := collStats["monitored_collections"].([]interface{}); ok {
				for _, coll := range monitored {
					if collStr, ok := coll.(string); ok {
						configMonitoredCollections = append(configMonitoredCollections, collStr)
					}
				}
			}
		}
	}

	// Log the configuration for debugging
	logger.Debug("Collection stats collector configuration",
		zap.Strings("monitored_collections", configMonitoredCollections),
		zap.Strings("enabled_metrics", config.EnabledMetrics))

	descriptors := map[string]*prometheus.Desc{
		"collection_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_size_bytes",
			"The total size of all records in the collection in bytes",
			labels,
			nil,
		),
		"collection_storage_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_storage_size_bytes",
			"Total amount of storage allocated to the collection in bytes",
			labels,
			nil,
		),
		"collection_avg_obj_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_avg_obj_size_bytes",
			"Average object size in the collection in bytes",
			labels,
			nil,
		),
		"collection_count": prometheus.NewDesc(
			"mongodb_collstats_count",
			"Number of documents in the collection",
			labels,
			nil,
		),
		"collection_indexes_count": prometheus.NewDesc(
			"mongodb_collstats_indexes_count",
			"Number of indexes in the collection",
			labels,
			nil,
		),
		"collection_total_index_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_total_index_size_bytes",
			"Total size of all indexes in the collection in bytes",
			labels,
			nil,
		),
		"collection_index_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_index_size_bytes",
			"Size of specific index in bytes",
			indexLabels,
			nil,
		),
		"collection_capped": prometheus.NewDesc(
			"mongodb_collstats_capped",
			"Whether the collection is capped (1) or not (0)",
			labels,
			nil,
		),
		"collection_max_documents": prometheus.NewDesc(
			"mongodb_collstats_max_documents",
			"Maximum number of documents in capped collection",
			labels,
			nil,
		),
		"collection_max_size_bytes": prometheus.NewDesc(
			"mongodb_collstats_max_size_bytes",
			"Maximum size of capped collection in bytes",
			labels,
			nil,
		),
		"collection_wiredtiger_cache_bytes": prometheus.NewDesc(
			"mongodb_collstats_wiredtiger_cache_bytes",
			"WiredTiger cache usage for collection in bytes",
			labels,
			nil,
		),
		"collection_wiredtiger_block_checkpoint_bytes": prometheus.NewDesc(
			"mongodb_collstats_wiredtiger_block_checkpoint_bytes",
			"WiredTiger block manager checkpoint bytes for collection",
			labels,
			nil,
		),
		"collection_wiredtiger_compression_ratio": prometheus.NewDesc(
			"mongodb_collstats_wiredtiger_compression_ratio",
			"WiredTiger compression ratio for collection",
			labels,
			nil,
		),
		"collection_ops_total": prometheus.NewDesc(
			"mongodb_collstats_ops_total",
			"Total number of operations performed on the collection",
			append(labels, "operation"),
			nil,
		),
		"collection_latency_microseconds": prometheus.NewDesc(
			"mongodb_collstats_latency_microseconds",
			"Average latency for operations on the collection in microseconds",
			append(labels, "operation"),
			nil,
		),
		"collection_read_concern_counters": prometheus.NewDesc(
			"mongodb_collstats_read_concern_counters",
			"Read concern usage counters for collection",
			append(labels, "read_concern"),
			nil,
		),
	}

	// Parse monitored collections from config if provided
	var monitoredCollections []string
	if len(config.EnabledMetrics) > 0 {
		// Check if specific collections are configured
		for _, metric := range config.EnabledMetrics {
			if metric == "collstats" || metric == "collection_stats" {
				// Use configMonitoredCollections if available, otherwise use all collections
				if len(configMonitoredCollections) > 0 {
					monitoredCollections = configMonitoredCollections
				} else {
					// Use all collections by default
					monitoredCollections = []string{"*"}
				}
				break
			}
		}
	}

	return &CollStatsCollector{
		BaseCollector:        NewBaseCollector(client, logger, config),
		descriptors:          descriptors,
		monitoredCollections: monitoredCollections,
	}
}

func (c *CollStatsCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("Collection stats collector starting")

	if !c.isMetricEnabled("collstats") && !c.isMetricEnabled("collection_stats") {
		c.logger.Debug("Collection stats collector disabled - not in enabled metrics")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get list of databases with optimized timeout
	databases, err := getDatabasesWithTimeout(ctx, c.client, 10*time.Second)
	if err != nil {
		c.logger.Error("Failed to list databases", zap.Error(err))
		return
	}

	c.logger.Debug("Found databases", zap.Strings("databases", databases))

	instance := c.getInstanceInfo(bson.M{})

	for _, dbName := range databases {
		// Skip system databases unless explicitly requested
		if c.shouldSkipDatabase(dbName) {
			c.logger.Debug("Skipping database", zap.String("database", dbName))
			continue
		}

		c.logger.Debug("Processing database", zap.String("database", dbName))
		c.collectDatabaseCollectionStats(ctx, ch, dbName, instance)
	}

	c.logger.Debug("Collection stats collector completed")
}

func (c *CollStatsCollector) collectDatabaseCollectionStats(ctx context.Context, ch chan<- prometheus.Metric, dbName string, instance map[string]string) {
	db := c.client.Database(dbName)

	// Get list of collections with optimized timeout
	collections, err := getCollectionsWithTimeout(ctx, db, 10*time.Second)
	if err != nil {
		c.logger.Error("Failed to list collections",
			zap.String("database", dbName),
			zap.Error(err))
		return
	}

	c.logger.Debug("Found collections", zap.String("database", dbName), zap.Strings("collections", collections))

	for _, collName := range collections {
		// Skip system collections unless explicitly requested
		if c.shouldSkipCollection(collName) {
			c.logger.Debug("Skipping collection", zap.String("database", dbName), zap.String("collection", collName))
			continue
		}

		// Check if this collection is in the monitored list
		if !c.shouldMonitorCollection(dbName, collName) {
			c.logger.Debug("Collection not in monitored list", zap.String("database", dbName), zap.String("collection", collName))
			continue
		}

		c.logger.Debug("Processing collection", zap.String("database", dbName), zap.String("collection", collName))
		c.collectCollectionStats(ctx, ch, dbName, collName, instance)
	}
}

func (c *CollStatsCollector) collectCollectionStats(ctx context.Context, ch chan<- prometheus.Metric, dbName, collName string, instance map[string]string) {
	var stats bson.M
	err := runCommandWithTimeout(ctx, c.client.Database(dbName), bson.D{
		{"collStats", collName},
	}, 10*time.Second, &stats)

	if err != nil {
		c.logger.Debug("Failed to get collection stats",
			zap.String("database", dbName),
			zap.String("collection", collName),
			zap.Error(err))
		return
	}

	c.collectBasicCollectionMetrics(ch, stats, dbName, collName, instance)
	c.collectIndexMetrics(ch, stats, dbName, collName, instance)
	c.collectWiredTigerMetrics(ch, stats, dbName, collName, instance)
	c.collectLatencyMetrics(ch, stats, dbName, collName, instance)
	c.collectReadConcernMetrics(ch, stats, dbName, collName, instance)
}

func (c *CollStatsCollector) collectBasicCollectionMetrics(ch chan<- prometheus.Metric, stats bson.M, dbName, collName string, instance map[string]string) {
	labels := []string{instance["instance"], instance["replica_set"], instance["shard"], dbName, collName}

	metrics := map[string]string{
		"size":           "collection_size_bytes",
		"storageSize":    "collection_storage_size_bytes",
		"avgObjSize":     "collection_avg_obj_size_bytes",
		"count":          "collection_count",
		"nindexes":       "collection_indexes_count",
		"totalIndexSize": "collection_total_index_size_bytes",
	}

	for statKey, descKey := range metrics {
		if value := c.getNumericValue(stats[statKey]); validateMetricValue(value) {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors[descKey],
				prometheus.GaugeValue,
				*value,
				labels...,
			)
		}
	}

	if capped, ok := stats["capped"].(bool); ok {
		cappedValue := 0.0
		if capped {
			cappedValue = 1.0
		}
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["collection_capped"],
			prometheus.GaugeValue,
			cappedValue,
			labels...,
		)

		if capped {
			cappedMetrics := map[string]string{
				"max":     "collection_max_documents",
				"maxSize": "collection_max_size_bytes",
			}
			for statKey, descKey := range cappedMetrics {
				if value := c.getNumericValue(stats[statKey]); value != nil {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors[descKey],
						prometheus.GaugeValue,
						*value,
						labels...,
					)
				}
			}
		}
	}
}

func (c *CollStatsCollector) collectIndexMetrics(ch chan<- prometheus.Metric, stats bson.M, dbName, collName string, instance map[string]string) {
	if indexSizes, ok := stats["indexSizes"].(bson.M); ok {
		for indexName, size := range indexSizes {
			if sizeValue, ok := size.(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_index_size_bytes"],
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

func (c *CollStatsCollector) collectWiredTigerMetrics(ch chan<- prometheus.Metric, stats bson.M, dbName, collName string, instance map[string]string) {
	if wiredTiger, ok := stats["wiredTiger"].(bson.M); ok {
		labels := []string{instance["instance"], instance["replica_set"], instance["shard"], dbName, collName}

		// Cache metrics
		if cache, ok := wiredTiger["cache"].(bson.M); ok {
			if cacheBytes, ok := cache["bytes currently in the cache"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_wiredtiger_cache_bytes"],
					prometheus.GaugeValue,
					float64(cacheBytes),
					labels...,
				)
			}
		}

		// Block manager metrics
		if blockManager, ok := wiredTiger["block-manager"].(bson.M); ok {
			if checkpointSize, ok := blockManager["checkpoint size"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_wiredtiger_block_checkpoint_bytes"],
					prometheus.GaugeValue,
					float64(checkpointSize),
					labels...,
				)
			}
		}

		// Compression metrics
		if compression, ok := wiredTiger["compression"].(bson.M); ok {
			if ratio, ok := compression["compression ratio"].(float64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_wiredtiger_compression_ratio"],
					prometheus.GaugeValue,
					ratio,
					labels...,
				)
			}
		}
	}
}

func (c *CollStatsCollector) collectLatencyMetrics(ch chan<- prometheus.Metric, stats bson.M, dbName, collName string, instance map[string]string) {
	if latencyStats, ok := stats["latencyStats"].(bson.M); ok {
		operations := []string{"reads", "writes", "commands"}

		for _, operation := range operations {
			if opStats, ok := latencyStats[operation].(bson.M); ok {
				if ops, ok := opStats["ops"].(int64); ok && ops > 0 {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["collection_ops_total"],
						prometheus.CounterValue,
						float64(ops),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						dbName,
						collName,
						operation,
					)
				}

				if latency, ok := opStats["latency"].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						c.descriptors["collection_latency_microseconds"],
						prometheus.GaugeValue,
						float64(latency),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						dbName,
						collName,
						operation,
					)
				}
			}
		}
	}
}

func (c *CollStatsCollector) collectReadConcernMetrics(ch chan<- prometheus.Metric, stats bson.M, dbName, collName string, instance map[string]string) {
	if readConcern, ok := stats["readConcern"].(bson.M); ok {
		readConcernLevels := []string{"local", "available", "majority", "linearizable", "snapshot"}

		for _, level := range readConcernLevels {
			if count, ok := readConcern[level].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_read_concern_counters"],
					prometheus.CounterValue,
					float64(count),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
					level,
				)
			}
		}
	}
}

func (c *CollStatsCollector) shouldSkipDatabase(dbName string) bool {
	return shouldSkipDatabase(dbName)
}

func (c *CollStatsCollector) shouldSkipCollection(collName string) bool {
	return shouldSkipCollection(collName)
}

func (c *CollStatsCollector) shouldMonitorCollection(dbName, collName string) bool {
	// If no specific collections configured, monitor all non-system collections
	if len(c.monitoredCollections) == 0 {
		return true
	}

	// Check if this specific collection is in the monitored list
	fullName := dbName + "." + collName
	for _, monitored := range c.monitoredCollections {
		if monitored == fullName || monitored == "*" {
			return true
		}
	}

	return false
}

func (c *CollStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *CollStatsCollector) Name() string {
	return "collstats"
}

// SetMonitoredCollections allows setting specific collections to monitor
func (c *CollStatsCollector) SetMonitoredCollections(collections []string) {
	c.monitoredCollections = collections
}
 