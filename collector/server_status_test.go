package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

func TestServerStatusCollector(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		EnabledMetrics: []string{"server_status"},
	}

	collector := NewServerStatusCollector(client, logger, config)

	if collector.BaseCollector == nil {
		t.Error("ServerStatusCollector should have BaseCollector")
	}

	if collector.descriptors == nil {
		t.Error("ServerStatusCollector should have descriptors")
	}

	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("ServerStatusCollector should collect metrics")
	}

	descCh := make(chan *prometheus.Desc, 10)
	collector.Describe(descCh)
	close(descCh)

	descCount := 0
	for range descCh {
		descCount++
	}

	if descCount == 0 {
		t.Error("ServerStatusCollector should describe metrics")
	}

	if collector.Name() != "server_status" {
		t.Error("ServerStatusCollector should have correct name")
	}
}

func TestServerStatusCollectorDisabled(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		EnabledMetrics: []string{"other_metric"},
	}

	collector := NewServerStatusCollector(client, logger, config)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count != 0 {
		t.Error("Disabled collector should not collect metrics")
	}
}

func TestServerStatusCollectorWithMockData(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		EnabledMetrics: []string{"server_status"},
	}

	collector := NewServerStatusCollector(client, logger, config)

	mockData := bson.M{
		"host":   "test-host",
		"uptime": int64(3600),
		"connections": bson.M{
			"current":      int32(10),
			"available":    int32(100),
			"active":       int32(5),
			"totalCreated": int64(1000),
		},
		"mem": bson.M{
			"resident": int64(1024 * 1024 * 100),
			"virtual":  int64(1024 * 1024 * 200),
		},
		"network": bson.M{
			"bytesIn":  int64(1024 * 1024),
			"bytesOut": int64(2048 * 1024),
		},
		"opcounters": bson.M{
			"insert":  int64(100),
			"query":   int64(500),
			"update":  int64(50),
			"delete":  int64(10),
			"getmore": int64(20),
			"command": int64(200),
		},
		"metrics": bson.M{
			"document": bson.M{
				"deleted":  int64(5),
				"inserted": int64(100),
				"returned": int64(500),
				"updated":  int64(50),
			},
		},
		"extra_info": bson.M{
			"page_faults": int64(10),
		},
	}

	ch := make(chan prometheus.Metric, 100)
	collector.collectMetrics(context.Background(), ch, mockData)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("Should collect metrics from mock data")
	}
}
