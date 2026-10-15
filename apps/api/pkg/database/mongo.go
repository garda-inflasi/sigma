package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoConfig holds MongoDB connection configuration.
type MongoConfig struct {
	URI      string
	DBName   string
}

// DefaultMongoConfig returns a MongoConfig populated from environment variables.
func DefaultMongoConfig() MongoConfig {
	return MongoConfig{
		URI:    getEnvOrDefault("MONGO_URI", "mongodb://localhost:27017"),
		DBName: getEnvOrDefault("MONGO_DB", "garda_inflasi"),
	}
}

// NewMongo creates a new MongoDB client connection.
func NewMongo(cfg MongoConfig) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return client, nil
}

// GetMongoCollection returns a handle to a MongoDB collection.
func GetMongoCollection(client *mongo.Client, dbName, collection string) *mongo.Collection {
	return client.Database(dbName).Collection(collection)
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
