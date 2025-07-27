package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type ShardingCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewShardingCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *ShardingCollector {
	labels := []string{"instance", "replica_set", "shard"}
	shardLabels := append(labels, "shard_name", "shard_host")
	chunkLabels := append(labels, "database", "collection", "shard_name")

	descriptors := map[string]*prometheus.Desc{
		"mongos_up": prometheus.NewDesc(
			"mongodb_mongos_up",
			"Whether the mongos instance is up",
			labels,
			nil,
		),
		"shards_total": prometheus.NewDesc(
			"mongodb_shards_total",
			"Total number of shards in the cluster",
			labels,
			nil,
		),
		"shard_chunks_total": prometheus.NewDesc(
			"mongodb_shard_chunks_total",
			"Total number of chunks per shard",
			chunkLabels,
			nil,
		),
		"balancer_enabled": prometheus.NewDesc(
			"mongodb_balancer_enabled",
			"Whether the balancer is enabled (1) or disabled (0)",
			labels,
			nil,
		),
		"balancer_running": prometheus.NewDesc(
			"mongodb_balancer_running",
			"Whether the balancer is currently running (1) or not (0)",
			labels,
			nil,
		),
		"balancer_migrations_total": prometheus.NewDesc(
			"mongodb_balancer_migrations_total",
			"Total number of chunk migrations",
			append(labels, "type"),
			nil,
		),
		"shard_databases_total": prometheus.NewDesc(
			"mongodb_shard_databases_total",
			"Number of databases on each shard",
			shardLabels,
			nil,
		),
		"shard_collections_total": prometheus.NewDesc(
			"mongodb_shard_collections_total",
			"Number of sharded collections per shard",
			shardLabels,
			nil,
		),
		"sharded_collections_total": prometheus.NewDesc(
			"mongodb_sharded_collections_total",
			"Total number of sharded collections in the cluster",
			labels,
			nil,
		),
		"chunk_migrations_failed_total": prometheus.NewDesc(
			"mongodb_chunk_migrations_failed_total",
			"Total number of failed chunk migrations",
			labels,
			nil,
		),
		"chunk_splits_total": prometheus.NewDesc(
			"mongodb_chunk_splits_total",
			"Total number of chunk splits",
			labels,
			nil,
		),
		"orphaned_documents": prometheus.NewDesc(
			"mongodb_orphaned_documents",
			"Number of orphaned documents per shard",
			shardLabels,
			nil,
		),
	}

	return &ShardingCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *ShardingCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("sharding") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Check if this is a mongos instance
	var isMaster bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"isMaster", 1}}).Decode(&isMaster)
	if err != nil {
		c.logger.Error("Failed to run isMaster command", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(isMaster)

	// Check if this is a mongos
	if msg, ok := isMaster["msg"].(string); ok && msg == "isdbgrid" {
		// This is a mongos, collect sharding metrics
		c.collectShardingMetrics(ctx, ch, instance)
	} else {
		// Not a mongos, skip sharding metrics
		c.logger.Debug("Not a mongos instance, skipping sharding metrics")
		return
	}
}

func (c *ShardingCollector) collectShardingMetrics(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Mongos is up
	ch <- prometheus.MustNewConstMetric(
		c.descriptors["mongos_up"],
		prometheus.GaugeValue,
		1.0,
		instance["instance"],
		instance["replica_set"],
		instance["shard"],
	)

	// Get shard information
	c.collectShardInfo(ctx, ch, instance)

	// Get balancer status
	c.collectBalancerStatus(ctx, ch, instance)

	// Get chunk distribution
	c.collectChunkDistribution(ctx, ch, instance)

	// Get database and collection statistics
	c.collectDatabaseShardDistribution(ctx, ch, instance)

	// Get migration statistics
	c.collectMigrationStats(ctx, ch, instance)
}

func (c *ShardingCollector) collectShardInfo(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// List shards
	cursor, err := c.client.Database("config").Collection("shards").Find(ctx, bson.D{})
	if err != nil {
		c.logger.Error("Failed to query config.shards", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var shards []bson.M
	if err := cursor.All(ctx, &shards); err != nil {
		c.logger.Error("Failed to decode shards", zap.Error(err))
		return
	}

	// Total number of shards
	ch <- prometheus.MustNewConstMetric(
		c.descriptors["shards_total"],
		prometheus.GaugeValue,
		float64(len(shards)),
		instance["instance"],
		instance["replica_set"],
		instance["shard"],
	)

	// Per-shard metrics
	for _, shard := range shards {
		shardName, ok1 := shard["_id"].(string)
		shardHost, ok2 := shard["host"].(string)

		if !ok1 || !ok2 {
			c.logger.Warn("Invalid shard data", zap.Any("shard", shard))
			continue
		}

		// Count databases per shard
		c.countDatabasesPerShard(ctx, ch, instance, shardName, shardHost)
	}
}

func (c *ShardingCollector) collectBalancerStatus(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Check balancer status
	var balancerStatus bson.M
	err := c.client.Database("admin").RunCommand(ctx, bson.D{{"balancerStatus", 1}}).Decode(&balancerStatus)
	if err != nil {
		c.logger.Error("Failed to get balancer status", zap.Error(err))
		return
	}

	// Balancer enabled status
	if mode, ok := balancerStatus["mode"].(string); ok {
		enabled := 0.0
		if mode != "off" {
			enabled = 1.0
		}
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["balancer_enabled"],
			prometheus.GaugeValue,
			enabled,
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}

	// Balancer running status
	if inBalancerRound, ok := balancerStatus["inBalancerRound"].(bool); ok {
		running := 0.0
		if inBalancerRound {
			running = 1.0
		}
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["balancer_running"],
			prometheus.GaugeValue,
			running,
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}
}

func (c *ShardingCollector) collectChunkDistribution(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Get chunk distribution from config.chunks
	pipeline := []bson.D{
		{{"$group", bson.D{
			{"_id", bson.D{
				{"ns", "$ns"},
				{"shard", "$shard"},
			}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
	}

	cursor, err := c.client.Database("config").Collection("chunks").Aggregate(ctx, pipeline)
	if err != nil {
		c.logger.Error("Failed to aggregate chunks", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		c.logger.Error("Failed to decode chunk distribution", zap.Error(err))
		return
	}

	for _, result := range results {
		id, ok := result["_id"].(bson.M)
		if !ok {
			continue
		}

		ns, ok1 := id["ns"].(string)
		shardName, ok2 := id["shard"].(string)
		count, ok3 := result["count"].(int32)

		if !ok1 || !ok2 || !ok3 {
			continue
		}

		// Parse namespace into database and collection
		db, collection := parseNamespace(ns)

		ch <- prometheus.MustNewConstMetric(
			c.descriptors["shard_chunks_total"],
			prometheus.GaugeValue,
			float64(count),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
			db,
			collection,
			shardName,
		)
	}
}

func (c *ShardingCollector) collectDatabaseShardDistribution(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Count sharded collections
	cursor, err := c.client.Database("config").Collection("collections").Find(ctx, bson.D{})
	if err != nil {
		c.logger.Error("Failed to query config.collections", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var collections []bson.M
	if err := cursor.All(ctx, &collections); err != nil {
		c.logger.Error("Failed to decode collections", zap.Error(err))
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.descriptors["sharded_collections_total"],
		prometheus.GaugeValue,
		float64(len(collections)),
		instance["instance"],
		instance["replica_set"],
		instance["shard"],
	)
}

func (c *ShardingCollector) collectMigrationStats(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Get migration history from config.changelog
	pipeline := []bson.D{
		{{"$match", bson.D{
			{"what", bson.D{{"$in", []string{"moveChunk.from", "moveChunk.to", "moveChunk.commit"}}}},
		}}},
		{{"$group", bson.D{
			{"_id", "$what"},
			{"count", bson.D{{"$sum", 1}}},
		}}},
	}

	cursor, err := c.client.Database("config").Collection("changelog").Aggregate(ctx, pipeline)
	if err != nil {
		c.logger.Debug("Failed to query config.changelog", zap.Error(err))
		return // This collection might not exist in older versions
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		c.logger.Error("Failed to decode migration stats", zap.Error(err))
		return
	}

	for _, result := range results {
		migType, ok1 := result["_id"].(string)
		count, ok2 := result["count"].(int32)

		if !ok1 || !ok2 {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.descriptors["balancer_migrations_total"],
			prometheus.CounterValue,
			float64(count),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
			migType,
		)
	}
}

func (c *ShardingCollector) countDatabasesPerShard(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string, shardName, shardHost string) {
	// Count databases on this shard
	cursor, err := c.client.Database("config").Collection("databases").Find(ctx, bson.D{
		{"primary", shardName},
	})
	if err != nil {
		c.logger.Error("Failed to query config.databases", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var databases []bson.M
	if err := cursor.All(ctx, &databases); err != nil {
		c.logger.Error("Failed to decode databases", zap.Error(err))
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.descriptors["shard_databases_total"],
		prometheus.GaugeValue,
		float64(len(databases)),
		instance["instance"],
		instance["replica_set"],
		instance["shard"],
		shardName,
		shardHost,
	)
}

func parseNamespace(ns string) (database, collection string) {
	// Split namespace like "db.collection" into database and collection
	for i, ch := range ns {
		if ch == '.' {
			return ns[:i], ns[i+1:]
		}
	}
	return ns, ""
}

func (c *ShardingCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *ShardingCollector) Name() string {
	return "sharding"
}
 