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

type ReplicaSetCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewReplicaSetCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *ReplicaSetCollector {
	labels := []string{"instance", "replica_set", "shard"}
	memberLabels := append(labels, "name", "state")

	descriptors := map[string]*prometheus.Desc{
		"member_state": prometheus.NewDesc(
			"mongodb_replset_member_state",
			"State of the replica set member (1=Primary, 2=Secondary, 7=Arbiter)",
			memberLabels,
			nil,
		),
		"member_health": prometheus.NewDesc(
			"mongodb_replset_member_health",
			"Health status of the replica set member (0=unhealthy, 1=healthy)",
			memberLabels,
			nil,
		),
		"number_of_members": prometheus.NewDesc(
			"mongodb_replset_number_of_members",
			"Total number of members in the replica set",
			labels,
			nil,
		),
		"oplog_size_bytes": prometheus.NewDesc(
			"mongodb_replset_oplog_size_bytes",
			"Size of the oplog in bytes",
			labels,
			nil,
		),
		"oplog_head_timestamp": prometheus.NewDesc(
			"mongodb_replset_oplog_head_timestamp",
			"Timestamp of the newest oplog entry",
			labels,
			nil,
		),
	}

	return &ReplicaSetCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *ReplicaSetCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("replica_set_status") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get replica set status
	var replStatus bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}}).Decode(&replStatus); err != nil {
		// If not a replica set, log at debug level and return
		if err.Error() == "not running with --replSet" {
			c.logger.Debug("Not running as replica set")
			return
		}
		c.logger.Error("Failed to get replica set status", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(replStatus)

	// Number of members
	if members, ok := replStatus["members"].(bson.A); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["number_of_members"],
			prometheus.GaugeValue,
			float64(len(members)),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)

		// Member state and health
		for _, m := range members {
			if member, ok := m.(bson.M); ok {
				name, ok1 := member["name"].(string)
				state, ok2 := member["state"].(int32)
				health, ok3 := member["health"].(int32)

				if !ok1 || !ok2 || !ok3 {
					c.logger.Warn("Invalid member data",
						zap.Any("member", member),
						zap.Bool("has_name", ok1),
						zap.Bool("has_state", ok2),
						zap.Bool("has_health", ok3))
					continue
				}

				ch <- prometheus.MustNewConstMetric(
					c.descriptors["member_state"],
					prometheus.GaugeValue,
					float64(state),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					name,
					c.getStateString(float64(state)),
				)

				ch <- prometheus.MustNewConstMetric(
					c.descriptors["member_health"],
					prometheus.GaugeValue,
					float64(health),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
					name,
					c.getStateString(float64(state)),
				)
			}
		}
	}

	// Oplog metrics
	c.collectOplogMetrics(ctx, ch, instance)
}

func (c *ReplicaSetCollector) collectOplogMetrics(ctx context.Context, ch chan<- prometheus.Metric, instance map[string]string) {
	// Get oplog size
	var oplogStats bson.M
	if err := c.client.Database("local").RunCommand(ctx, bson.D{{"collStats", "oplog.rs"}}).Decode(&oplogStats); err != nil {
		c.logger.Debug("Failed to get oplog stats", zap.Error(err))
		return
	}

	if size, ok := oplogStats["size"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["oplog_size_bytes"],
			prometheus.GaugeValue,
			float64(size),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}

	// Get latest oplog entry timestamp
	var latestOplog bson.M
	opts := options.FindOne().SetSort(bson.D{{"$natural", -1}})
	if err := c.client.Database("local").Collection("oplog.rs").FindOne(ctx, bson.M{}, opts).Decode(&latestOplog); err != nil {
		c.logger.Debug("Failed to get latest oplog entry", zap.Error(err))
		return
	}

	if ts, ok := latestOplog["ts"].(primitive.Timestamp); ok {
		ch <- prometheus.MustNewConstMetric(
			c.descriptors["oplog_head_timestamp"],
			prometheus.GaugeValue,
			float64(ts.T),
			instance["instance"],
			instance["replica_set"],
			instance["shard"],
		)
	}
}

func (c *ReplicaSetCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *ReplicaSetCollector) Name() string {
	return "replica_set_status"
}

func (c *ReplicaSetCollector) getStateString(state float64) string {
	switch state {
	case 1:
		return "PRIMARY"
	case 2:
		return "SECONDARY"
	case 7:
		return "ARBITER"
	default:
		return "UNKNOWN"
	}
}
