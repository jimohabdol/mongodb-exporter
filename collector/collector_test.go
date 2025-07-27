package collector

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func setupTestMongoDB(t *testing.T) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	return client
}

func TestBaseCollector(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		CustomLabels:    map[string]string{"test": "value"},
		EnabledMetrics:  []string{"server_status"},
		DisabledMetrics: []string{"profile"},
		Collectors:      map[string]interface{}{},
	}

	bc := NewBaseCollector(client, logger, config)

	if bc.client == nil {
		t.Error("BaseCollector client should not be nil")
	}

	if bc.logger == nil {
		t.Error("BaseCollector logger should not be nil")
	}

	instance := bc.getInstanceInfo(bson.M{
		"host":  "test-host",
		"repl":  bson.M{"setName": "test-replica"},
		"shard": "test-shard",
	})

	if instance["instance"] != "test-host" {
		t.Errorf("Expected instance 'test-host', got '%s'", instance["instance"])
	}

	if instance["replica_set"] != "test-replica" {
		t.Errorf("Expected replica_set 'test-replica', got '%s'", instance["replica_set"])
	}

	if instance["shard"] != "test-shard" {
		t.Errorf("Expected shard 'test-shard', got '%s'", instance["shard"])
	}

	if !bc.isMetricEnabled("server_status") {
		t.Error("server_status should be enabled")
	}

	if bc.isMetricEnabled("profile") {
		t.Error("profile should be disabled")
	}

	if bc.isMetricEnabled("unknown_metric") {
		t.Error("unknown metric should be disabled when specific enabled metrics are set")
	}

	config.EnabledMetrics = []string{"specific_metric"}
	bc = NewBaseCollector(client, logger, config)

	if bc.isMetricEnabled("server_status") {
		t.Error("server_status should be disabled when specific metrics are enabled")
	}

	if !bc.isMetricEnabled("specific_metric") {
		t.Error("specific_metric should be enabled")
	}

	labels := prometheus.Labels{"label1": "value1"}
	bc.addCustomLabels(labels)

	if labels["test"] != "value" {
		t.Error("Custom labels should be added")
	}

	value := bc.getNumericValue(int64(100))
	if value == nil || *value != 100.0 {
		t.Error("getNumericValue should return correct float64 value")
	}

	value = bc.getNumericValue(int32(50))
	if value == nil || *value != 50.0 {
		t.Error("getNumericValue should handle int32")
	}

	value = bc.getNumericValue(25)
	if value == nil || *value != 25.0 {
		t.Error("getNumericValue should handle int")
	}

	value = bc.getNumericValue(12.5)
	if value == nil || *value != 12.5 {
		t.Error("getNumericValue should handle float64")
	}

	value = bc.getNumericValue("invalid")
	if value != nil {
		t.Error("getNumericValue should return nil for invalid types")
	}

	value = bc.getNumericValue(-1)
	if value != nil {
		t.Error("getNumericValue should return nil for negative values")
	}
}

func TestMultiCollector(t *testing.T) {
	logger := zap.NewNop()
	mc := NewMultiCollector(logger)

	if mc.logger == nil {
		t.Error("MultiCollector logger should not be nil")
	}

	collector := &MockCollector{name: "test"}
	mc.AddCollector(collector)

	if len(mc.collectors) != 1 {
		t.Error("Collector should be added")
	}

	mc.RemoveCollector("test")
	if len(mc.collectors) != 0 {
		t.Error("Collector should be removed")
	}

	mc.AddCollector(collector)
	ch := make(chan prometheus.Metric, 10)
	mc.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("MultiCollector should collect metrics")
	}

	descCh := make(chan *prometheus.Desc, 10)
	mc.Describe(descCh)
	close(descCh)

	count = 0
	for range descCh {
		count++
	}

	if count == 0 {
		t.Error("MultiCollector should describe metrics")
	}

	if mc.Name() != "multi_collector" {
		t.Error("MultiCollector should have correct name")
	}
}

func TestCollectorManager(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		EnabledMetrics: []string{"server_status"},
	}

	cm := NewCollectorManager(client, logger, config)

	if cm.client == nil {
		t.Error("CollectorManager client should not be nil")
	}

	if cm.logger == nil {
		t.Error("CollectorManager logger should not be nil")
	}

	if !cm.isMetricEnabled("server_status") {
		t.Error("server_status should be enabled")
	}

	if cm.isMetricEnabled("disabled_metric") {
		t.Error("disabled_metric should be disabled")
	}

	collector := cm.GetCollector()
	if collector == nil {
		t.Error("CollectorManager should return a collector")
	}

	ctx := cm.Context()
	if ctx == nil {
		t.Error("CollectorManager should return a context")
	}

	cm.Shutdown()
}

func TestInitializeCollectors(t *testing.T) {
	client := setupTestMongoDB(t)
	logger := zap.NewNop()
	config := CollectorConfig{
		EnabledMetrics: []string{"server_status", "replica_set_status"},
		Collectors: map[string]interface{}{
			"collstats": map[string]interface{}{
				"monitored_collections": []string{"test"},
			},
		},
	}

	collectors := InitializeCollectors(client, logger, config)

	if len(collectors) == 0 {
		t.Error("Should initialize collectors")
	}

	foundServerStatus := false
	for _, collector := range collectors {
		if collector.Name() == "server_status" {
			foundServerStatus = true
			break
		}
	}

	if !foundServerStatus {
		t.Error("Should initialize server_status collector")
	}
}

type MockCollector struct {
	name string
}

func (m *MockCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("mock_metric", "Mock metric", nil, nil)
}

func (m *MockCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("mock_metric", "Mock metric", nil, nil),
		prometheus.GaugeValue,
		1.0,
	)
}

func (m *MockCollector) Name() string {
	return m.name
}
