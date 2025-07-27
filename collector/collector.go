package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type Collector interface {
	prometheus.Collector
	Name() string
}

type BaseCollector struct {
	client *mongo.Client
	logger *zap.Logger
	config CollectorConfig
}

type CollectorConfig struct {
	CustomLabels    map[string]string
	EnabledMetrics  []string
	DisabledMetrics []string
	Collectors      map[string]interface{}
}

func NewBaseCollector(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *BaseCollector {
	return &BaseCollector{
		client: client,
		logger: logger,
		config: config,
	}
}

func (bc *BaseCollector) getInstanceInfo(result bson.M) map[string]string {
	instance := map[string]string{
		"instance":    "unknown",
		"replica_set": "unknown",
		"shard":       "unknown",
	}

	if host, ok := result["host"].(string); ok {
		instance["instance"] = host
	}

	if repl, ok := result["repl"].(bson.M); ok {
		if setName, ok := repl["setName"].(string); ok {
			instance["replica_set"] = setName
		}
	}

	if shard, ok := result["shard"].(string); ok {
		instance["shard"] = shard
	}

	return instance
}

func (bc *BaseCollector) isMetricEnabled(metricName string) bool {
	for _, disabled := range bc.config.DisabledMetrics {
		if disabled == metricName {
			return false
		}
	}

	if len(bc.config.EnabledMetrics) == 0 {
		return true
	}

	for _, enabled := range bc.config.EnabledMetrics {
		if enabled == metricName {
			return true
		}
	}

	return false
}

func (bc *BaseCollector) addCustomLabels(labels prometheus.Labels) {
	for key, value := range bc.config.CustomLabels {
		labels[key] = value
	}
}

func (bc *BaseCollector) getNumericValue(value interface{}) *float64 {
	return safeGetNumericValue(value)
}

type MultiCollector struct {
	collectors []Collector
	logger     *zap.Logger
	wg         sync.WaitGroup
	mu         sync.Mutex
	errors     []error
}

func NewMultiCollector(logger *zap.Logger) *MultiCollector {
	return &MultiCollector{
		collectors: make([]Collector, 0),
		logger:     logger,
	}
}

func (mc *MultiCollector) AddCollector(collector Collector) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.collectors = append(mc.collectors, collector)
	mc.logger.Info("Added collector", zap.String("name", collector.Name()))
}

func (mc *MultiCollector) RemoveCollector(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for i, collector := range mc.collectors {
		if collector.Name() == name {
			mc.collectors = append(mc.collectors[:i], mc.collectors[i+1:]...)
			mc.logger.Info("Removed collector", zap.String("name", name))
			return
		}
	}
}

func (mc *MultiCollector) Collect(ch chan<- prometheus.Metric) {
	mc.mu.Lock()
	collectors := make([]Collector, len(mc.collectors))
	copy(collectors, mc.collectors)
	mc.mu.Unlock()

	var errors []error
	var errorsMu sync.Mutex

	var wg sync.WaitGroup
	for _, collector := range collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errorsMu.Lock()
					errors = append(errors, fmt.Errorf("panic in collector %s: %v", c.Name(), r))
					errorsMu.Unlock()
					mc.logger.Error("Collector panicked",
						zap.String("collector", c.Name()),
						zap.Any("panic", r))
				}
			}()
			c.Collect(ch)
		}(collector)
	}

	wg.Wait()

	if len(errors) > 0 {
		mc.logger.Error("Errors occurred during collection",
			zap.Int("error_count", len(errors)),
			zap.Errors("errors", errors))
	}
}

func (mc *MultiCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range mc.collectors {
		collector.Describe(ch)
	}
}

func (mc *MultiCollector) Name() string {
	return "multi_collector"
}

type CollectorManager struct {
	multiCollector *MultiCollector
	logger         *zap.Logger
	client         *mongo.Client
	config         CollectorConfig
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewCollectorManager(client *mongo.Client, logger *zap.Logger, config CollectorConfig) *CollectorManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &CollectorManager{
		multiCollector: NewMultiCollector(logger),
		logger:         logger,
		client:         client,
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (cm *CollectorManager) isMetricEnabled(metricName string) bool {
	for _, disabled := range cm.config.DisabledMetrics {
		if disabled == metricName {
			return false
		}
	}

	if len(cm.config.EnabledMetrics) == 0 {
		return true
	}

	for _, enabled := range cm.config.EnabledMetrics {
		if enabled == metricName {
			return true
		}
	}

	return false
}

func InitializeCollectors(client *mongo.Client, logger *zap.Logger, config CollectorConfig) []Collector {
	collectors := []Collector{
		NewServerStatusCollector(client, logger, config),
		NewReplicaSetCollector(client, logger, config),
		NewQueryExecutorCollector(client, logger, config),
		NewWiredTigerCollector(client, logger, config),
		NewLockCollector(client, logger, config),
		NewIndexStatsCollector(client, logger, config),
		NewStorageStatsCollector(client, logger, config),
		NewCompatibilityCollector(client, logger, config),
		NewShardingCollector(client, logger, config),
		NewCollStatsCollector(client, logger, config),
		NewCursorCollector(client, logger, config),
		NewProfileCollector(client, logger, config),
		NewConnectionPoolCollector(client, logger, config),
	}

	return collectors
}

func (cm *CollectorManager) InitializeCollectors() error {
	collectors := InitializeCollectors(cm.client, cm.logger, cm.config)

	// Verify collectors before registering
	for _, collector := range collectors {
		if collector == nil {
			return fmt.Errorf("nil collector found")
		}
	}

	cm.multiCollector = &MultiCollector{
		collectors: collectors,
		logger:     cm.logger,
	}

	return nil
}

func (cm *CollectorManager) GetCollector() Collector {
	return cm.multiCollector
}

func (cm *CollectorManager) Shutdown() {
	cm.cancel()
	cm.logger.Info("Collector manager shutdown")
}

func (cm *CollectorManager) Context() context.Context {
	return cm.ctx
}
