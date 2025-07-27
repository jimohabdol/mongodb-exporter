package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type StorageStatsCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewStorageStatsCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *StorageStatsCollector {
	labels := []string{"instance", "replica_set", "shard", "database"}
	collectionLabels := append(labels, "collection")

	descriptors := map[string]*prometheus.Desc{
		"database_size_bytes": prometheus.NewDesc(
			"mongodb_database_size_bytes",
			"Total size of the database in bytes",
			labels,
			nil,
		),
		"collection_size_bytes": prometheus.NewDesc(
			"mongodb_collection_size_bytes",
			"Total size of the collection in bytes",
			collectionLabels,
			nil,
		),
		"collection_storage_size_bytes": prometheus.NewDesc(
			"mongodb_collection_storage_size_bytes",
			"Total storage size of the collection in bytes",
			collectionLabels,
			nil,
		),
		"collection_avg_obj_size_bytes": prometheus.NewDesc(
			"mongodb_collection_avg_obj_size_bytes",
			"Average object size in the collection in bytes",
			collectionLabels,
			nil,
		),
		"collection_count": prometheus.NewDesc(
			"mongodb_collection_count",
			"Number of documents in the collection",
			collectionLabels,
			nil,
		),
		"collection_index_size_bytes": prometheus.NewDesc(
			"mongodb_collection_index_size_bytes",
			"Total size of all indexes in the collection",
			collectionLabels,
			nil,
		),
		"collection_capped": prometheus.NewDesc(
			"mongodb_collection_capped",
			"Whether the collection is capped (1) or not (0)",
			collectionLabels,
			nil,
		),
	}

	return &StorageStatsCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *StorageStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("storage_stats") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get list of databases
	databases, err := c.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		c.logger.Error("Failed to list databases", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(bson.M{})

	for _, dbName := range databases {
		// Skip admin and local databases
		if dbName == "admin" || dbName == "local" || dbName == "config" {
			continue
		}

		// Get database stats
		var dbStats bson.M
		if err := c.client.Database(dbName).RunCommand(ctx, bson.D{{"dbStats", 1}}).Decode(&dbStats); err != nil {
			c.logger.Error("Failed to get database stats",
				zap.String("database", dbName),
				zap.Error(err))
			continue
		}

		// Database size
		if dataSize, ok := dbStats["dataSize"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(
				c.descriptors["database_size_bytes"],
				prometheus.GaugeValue,
				float64(dataSize),
				instance["instance"],
				instance["replica_set"],
				instance["shard"],
				dbName,
			)
		}

		// Get collections
		db := c.client.Database(dbName)
		collections, err := db.ListCollectionNames(ctx, bson.D{})
		if err != nil {
			c.logger.Error("Failed to list collections",
				zap.String("database", dbName),
				zap.Error(err))
			continue
		}

		for _, collName := range collections {
			var collStats bson.M
			if err := db.RunCommand(ctx, bson.D{{"collStats", collName}}).Decode(&collStats); err != nil {
				c.logger.Error("Failed to get collection stats",
					zap.String("database", dbName),
					zap.String("collection", collName),
					zap.Error(err))
				continue
			}

			// Collection metrics
			metrics := map[string]string{
				"size":           "collection_size_bytes",
				"storageSize":    "collection_storage_size_bytes",
				"avgObjSize":     "collection_avg_obj_size_bytes",
				"count":          "collection_count",
				"totalIndexSize": "collection_index_size_bytes",
			}

			for statName, metricName := range metrics {
				if value, ok := collStats[statName].(int64); ok {
					if desc, ok := c.descriptors[metricName]; ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							float64(value),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							dbName,
							collName,
						)
					}
				}
			}

			// Capped collection status
			if capped, ok := collStats["capped"].(bool); ok {
				cappedValue := 0.0
				if capped {
					cappedValue = 1.0
				}
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["collection_capped"],
					prometheus.GaugeValue,
					cappedValue,
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					dbName,
					collName,
				)
			}
		}
	}
}

func (c *StorageStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *StorageStatsCollector) Name() string {
	return "storage_stats"
}
