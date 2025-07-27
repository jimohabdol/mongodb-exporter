package collector

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Common database utilities to eliminate DRY violations

// shouldSkipDatabase checks if a database should be skipped during collection
func shouldSkipDatabase(dbName string) bool {
	systemDatabases := []string{"admin", "config", "local"}
	for _, sysDB := range systemDatabases {
		if dbName == sysDB {
			return true
		}
	}
	return false
}

// shouldSkipCollection checks if a collection should be skipped
func shouldSkipCollection(collName string) bool {
	return len(collName) > 7 && collName[:7] == "system."
}

// getDatabasesWithTimeout gets list of databases with timeout
func getDatabasesWithTimeout(ctx context.Context, client *mongo.Client, timeout time.Duration) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return client.ListDatabaseNames(timeoutCtx, bson.D{})
}

// getCollectionsWithTimeout gets list of collections with timeout
func getCollectionsWithTimeout(ctx context.Context, db *mongo.Database, timeout time.Duration) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return db.ListCollectionNames(timeoutCtx, bson.D{})
}

// runCommandWithTimeout runs a MongoDB command with timeout
func runCommandWithTimeout(ctx context.Context, db *mongo.Database, command bson.D, timeout time.Duration, result interface{}) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return db.RunCommand(timeoutCtx, command).Decode(result)
}

// validateMetricValue ensures metric values are valid
func validateMetricValue(value *float64) bool {
	if value == nil {
		return false
	}
	if *value < 0 {
		return false
	}
	return true
}

// safeGetNumericValue safely extracts numeric values from BSON
func safeGetNumericValue(value interface{}) *float64 {
	switch v := value.(type) {
	case int64:
		if v < 0 {
			return nil
		}
		result := float64(v)
		return &result
	case int32:
		if v < 0 {
			return nil
		}
		result := float64(v)
		return &result
	case int:
		if v < 0 {
			return nil
		}
		result := float64(v)
		return &result
	case float64:
		if v < 0 {
			return nil
		}
		return &v
	default:
		return nil
	}
}
