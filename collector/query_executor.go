package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type QueryExecutorCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewQueryExecutorCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *QueryExecutorCollector {
	labels := []string{"instance", "replica_set", "shard"}
	descriptors := map[string]*prometheus.Desc{
		"query_executor_total": prometheus.NewDesc(
			"mongodb_metrics_query_executor_total",
			"Total number of query executor operations",
			labels,
			nil,
		),
		"scanned_total": prometheus.NewDesc(
			"mongodb_metrics_query_executor_scanned_total",
			"Total number of documents scanned by query executor",
			labels,
			nil,
		),
		"scanned_objects_total": prometheus.NewDesc(
			"mongodb_metrics_query_executor_scanned_objects_total",
			"Total number of objects scanned by query executor",
			labels,
			nil,
		),
	}

	return &QueryExecutorCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *QueryExecutorCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("query_executor") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect query executor metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	if metrics, ok := result["metrics"].(bson.M); ok {
		if queryExecutor, ok := metrics["queryExecutor"].(bson.M); ok {
			// Total queries
			if total, ok := queryExecutor["scanned"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["query_executor_total"],
					prometheus.CounterValue,
					float64(total),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}

			// Scanned documents
			if scanned, ok := queryExecutor["scanned"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["scanned_total"],
					prometheus.CounterValue,
					float64(scanned),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}

			// Scanned objects
			if scannedObjects, ok := queryExecutor["scannedObjects"].(int64); ok {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors["scanned_objects_total"],
					prometheus.CounterValue,
					float64(scannedObjects),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}
		}
	}
}

func (c *QueryExecutorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *QueryExecutorCollector) Name() string {
	return "query_executor"
}

func (c *QueryExecutorCollector) getInstanceInfo(result bson.M) prometheus.Labels {
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

func (c *QueryExecutorCollector) collectQueryExecutorMetrics(ch chan<- prometheus.Metric, result bson.M, labels prometheus.Labels) {
	if metrics, ok := result["metrics"].(bson.M); ok {
		if queryExecutor, ok := metrics["queryExecutor"].(bson.M); ok {
			c.collectScannedMetrics(ch, queryExecutor, labels)
			c.collectPlanCacheMetrics(ch, queryExecutor, labels)
		}
	}
}

func (c *QueryExecutorCollector) collectScannedMetrics(ch chan<- prometheus.Metric, queryExecutor bson.M, labels prometheus.Labels) {
	if scanned, ok := queryExecutor["scanned"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_scanned_total"], prometheus.CounterValue, float64(scanned), labels["instance"], labels["replica_set"], labels["shard"])
	}

	if scannedObjects, ok := queryExecutor["scannedObjects"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_scanned_objects_total"], prometheus.CounterValue, float64(scannedObjects), labels["instance"], labels["replica_set"], labels["shard"])
	}
}

func (c *QueryExecutorCollector) collectPlanCacheMetrics(ch chan<- prometheus.Metric, queryExecutor bson.M, labels prometheus.Labels) {
	if planCache, ok := queryExecutor["planCache"].(bson.M); ok {
		if hits, ok := planCache["hits"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_plan_cache_hits_total"], prometheus.CounterValue, float64(hits), labels["instance"], labels["replica_set"], labels["shard"])
		}

		if misses, ok := planCache["misses"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_plan_cache_misses_total"], prometheus.CounterValue, float64(misses), labels["instance"], labels["replica_set"], labels["shard"])
		}

		if evictions, ok := planCache["evictions"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_plan_cache_evictions_total"], prometheus.CounterValue, float64(evictions), labels["instance"], labels["replica_set"], labels["shard"])
		}

		if entries, ok := planCache["entries"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_plan_cache_entries"], prometheus.GaugeValue, float64(entries), labels["instance"], labels["replica_set"], labels["shard"])
		}

		if size, ok := planCache["size"].(int64); ok {
			ch <- prometheus.MustNewConstMetric(c.descriptors["metrics_query_executor_plan_cache_size_bytes"], prometheus.GaugeValue, float64(size), labels["instance"], labels["replica_set"], labels["shard"])
		}
	}
}
