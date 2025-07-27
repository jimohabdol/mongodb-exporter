package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type WiredTigerCollector struct {
	*BaseCollector
	descriptors map[string]*prometheus.Desc
}

func NewWiredTigerCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *WiredTigerCollector {
	labels := []string{"instance", "replica_set", "shard"}
	cacheLabels := append(labels, "type")

	descriptors := map[string]*prometheus.Desc{
		"cache_max_bytes": prometheus.NewDesc(
			"mongodb_wiredtiger_cache_max_bytes",
			"Maximum bytes configured for cache",
			labels,
			nil,
		),
		"cache_used_bytes": prometheus.NewDesc(
			"mongodb_wiredtiger_cache_used_bytes",
			"Bytes currently in cache",
			labels,
			nil,
		),
		"cache_dirty_bytes": prometheus.NewDesc(
			"mongodb_wiredtiger_cache_dirty_bytes",
			"Bytes currently dirty in cache",
			labels,
			nil,
		),
		"cache_pages": prometheus.NewDesc(
			"mongodb_wiredtiger_cache_pages",
			"Number of pages by state",
			cacheLabels,
			nil,
		),
		"cache_evicted_total": prometheus.NewDesc(
			"mongodb_wiredtiger_cache_evicted_total",
			"Pages evicted from cache",
			append(labels, "mode"),
			nil,
		),
		"io_total": prometheus.NewDesc(
			"mongodb_wiredtiger_io_total",
			"Number of I/O operations",
			append(labels, "type"),
			nil,
		),
		"scan_total": prometheus.NewDesc(
			"mongodb_wiredtiger_scan_total",
			"Scan operations",
			append(labels, "type"),
			nil,
		),
		"block_operations_total": prometheus.NewDesc(
			"mongodb_wiredtiger_block_operations_total",
			"Block operations",
			append(labels, "type"),
			nil,
		),
	}

	return &WiredTigerCollector{
		BaseCollector: NewBaseCollector(client, logger, config),
		descriptors:   descriptors,
	}
}

func (c *WiredTigerCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isMetricEnabled("wiredtiger") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result bson.M
	if err := c.client.Database("admin").RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&result); err != nil {
		c.logger.Error("Failed to collect WiredTiger metrics", zap.Error(err))
		return
	}

	instance := c.getInstanceInfo(result)

	if wt, ok := result["wiredTiger"].(bson.M); ok {
		c.collectCacheMetrics(ch, wt, instance)
		c.collectBlockManagerMetrics(ch, wt, instance)
		c.collectConcurrentTransactionsMetrics(ch, wt, instance)
	}
}

func (c *WiredTigerCollector) collectCacheMetrics(ch chan<- prometheus.Metric, wt bson.M, instance map[string]string) {
	if cache, ok := wt["cache"].(bson.M); ok {
		// Maximum configured cache size
		if maxBytes, ok := cache["maximum bytes configured"].(int64); ok {
			if desc, ok := c.descriptors["cache_max_bytes"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					float64(maxBytes),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}
		}

		// Current cache usage
		if bytesInCache, ok := cache["bytes currently in the cache"].(int64); ok {
			if desc, ok := c.descriptors["cache_used_bytes"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					float64(bytesInCache),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}
		}

		// Dirty bytes in cache
		if dirtyBytes, ok := cache["tracked dirty bytes in the cache"].(int64); ok {
			if desc, ok := c.descriptors["cache_dirty_bytes"]; ok {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					float64(dirtyBytes),
					instance["instance"],
					instance["replica_set"],
					instance["shard"],
				)
			}
		}

		// Pages by state
		pageStates := map[string]string{
			"pages currently held in the cache": "total",
			"tracked dirty pages in the cache":  "dirty",
			"pages read into cache":             "read",
			"pages written from cache":          "written",
		}

		if desc, ok := c.descriptors["cache_pages"]; ok {
			for metric, label := range pageStates {
				if value, ok := cache[metric].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.GaugeValue,
						float64(value),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						label,
					)
				}
			}
		}

		// Evicted pages
		evictionTypes := map[string]string{
			"unmodified pages evicted": "clean",
			"modified pages evicted":   "dirty",
		}

		if desc, ok := c.descriptors["cache_evicted_total"]; ok {
			for metric, label := range evictionTypes {
				if value, ok := cache[metric].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.CounterValue,
						float64(value),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						label,
					)
				}
			}
		}
	}
}

func (c *WiredTigerCollector) collectBlockManagerMetrics(ch chan<- prometheus.Metric, wt bson.M, instance map[string]string) {
	if blockManager, ok := wt["block-manager"].(bson.M); ok {
		// Block operations
		blockOps := map[string]string{
			"blocks read":    "read",
			"blocks written": "written",
			"bytes read":     "bytes_read",
			"bytes written":  "bytes_written",
		}

		if desc, ok := c.descriptors["block_operations_total"]; ok {
			for metric, label := range blockOps {
				if value, ok := blockManager[metric].(int64); ok {
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.CounterValue,
						float64(value),
						instance["instance"],
						instance["replica_set"],
						instance["shard"],
						label,
					)
				}
			}
		}
	}
}

func (c *WiredTigerCollector) collectConcurrentTransactionsMetrics(ch chan<- prometheus.Metric, wt bson.M, instance map[string]string) {
	if concurrentTransactions, ok := wt["concurrentTransactions"].(bson.M); ok {
		if desc, ok := c.descriptors["io_total"]; ok {
			for txType, metrics := range concurrentTransactions {
				if metricsMap, ok := metrics.(bson.M); ok {
					if available, ok := metricsMap["available"].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							float64(available),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							txType+"_available",
						)
					}
					if out, ok := metricsMap["out"].(int64); ok {
						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							float64(out),
							instance["instance"],
							instance["replica_set"],
							instance["shard"],
							txType+"_used",
						)
					}
				}
			}
		}
	}
}

func (c *WiredTigerCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors {
		ch <- desc
	}
}

func (c *WiredTigerCollector) Name() string {
	return "wiredtiger"
}
