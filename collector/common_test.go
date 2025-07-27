package collector

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestShouldSkipDatabase(t *testing.T) {
	if !shouldSkipDatabase("admin") {
		t.Error("admin database should be skipped")
	}

	if !shouldSkipDatabase("config") {
		t.Error("config database should be skipped")
	}

	if !shouldSkipDatabase("local") {
		t.Error("local database should be skipped")
	}

	if shouldSkipDatabase("test") {
		t.Error("test database should not be skipped")
	}

	if shouldSkipDatabase("myapp") {
		t.Error("myapp database should not be skipped")
	}
}

func TestShouldSkipCollection(t *testing.T) {
	if !shouldSkipCollection("system.profile") {
		t.Error("system.profile should be skipped")
	}

	if !shouldSkipCollection("system.indexes") {
		t.Error("system.indexes should be skipped")
	}

	if !shouldSkipCollection("system.namespaces") {
		t.Error("system.namespaces should be skipped")
	}

	if shouldSkipCollection("users") {
		t.Error("users should not be skipped")
	}

	if shouldSkipCollection("orders") {
		t.Error("orders should not be skipped")
	}
}

func TestGetDatabasesWithTimeout(t *testing.T) {
	client := setupTestMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	databases, err := getDatabasesWithTimeout(ctx, client, 3*time.Second)
	if err != nil {
		t.Errorf("getDatabasesWithTimeout failed: %v", err)
	}

	if len(databases) == 0 {
		t.Error("Should return at least one database")
	}

	foundTest := false
	for _, db := range databases {
		if db == "test" {
			foundTest = true
			break
		}
	}

	if !foundTest {
		t.Error("Should find test database")
	}
}

func TestGetCollectionsWithTimeout(t *testing.T) {
	client := setupTestMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db := client.Database("test")
	collections, err := getCollectionsWithTimeout(ctx, db, 3*time.Second)
	if err != nil {
		t.Errorf("getCollectionsWithTimeout failed: %v", err)
	}

	if len(collections) == 0 {
		t.Error("Should return at least one collection")
	}
}

func TestRunCommandWithTimeout(t *testing.T) {
	client := setupTestMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db := client.Database("test")
	var result bson.M
	err := runCommandWithTimeout(ctx, db, bson.D{{"ping", 1}}, 3*time.Second, &result)
	if err != nil {
		t.Errorf("runCommandWithTimeout failed: %v", err)
	}

	if result["ok"] != float64(1) {
		t.Error("ping command should return ok: 1")
	}
}

func TestValidateMetricValue(t *testing.T) {
	value := 10.5
	if !validateMetricValue(&value) {
		t.Error("Valid positive value should be validated")
	}

	value = 0.0
	if !validateMetricValue(&value) {
		t.Error("Zero value should be validated")
	}

	value = -1.0
	if validateMetricValue(&value) {
		t.Error("Negative value should not be validated")
	}

	if validateMetricValue(nil) {
		t.Error("Nil value should not be validated")
	}
}

func TestSafeGetNumericValue(t *testing.T) {
	value := safeGetNumericValue(int64(100))
	if value == nil || *value != 100.0 {
		t.Error("int64 should be converted correctly")
	}

	value = safeGetNumericValue(int32(50))
	if value == nil || *value != 50.0 {
		t.Error("int32 should be converted correctly")
	}

	value = safeGetNumericValue(25)
	if value == nil || *value != 25.0 {
		t.Error("int should be converted correctly")
	}

	value = safeGetNumericValue(12.5)
	if value == nil || *value != 12.5 {
		t.Error("float64 should be converted correctly")
	}

	value = safeGetNumericValue("invalid")
	if value != nil {
		t.Error("string should return nil")
	}

	value = safeGetNumericValue(nil)
	if value != nil {
		t.Error("nil should return nil")
	}

	value = safeGetNumericValue(int64(-1))
	if value != nil {
		t.Error("negative value should return nil")
	}

	value = safeGetNumericValue(int32(-5))
	if value != nil {
		t.Error("negative int32 should return nil")
	}

	value = safeGetNumericValue(-10)
	if value != nil {
		t.Error("negative int should return nil")
	}

	value = safeGetNumericValue(-2.5)
	if value != nil {
		t.Error("negative float64 should return nil")
	}
}
