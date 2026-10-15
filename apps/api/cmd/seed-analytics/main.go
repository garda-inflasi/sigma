package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/garda-inflasi/api/internal/analytics/seeder"
	"github.com/garda-inflasi/api/pkg/database"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("MONGO_DATABASE")
	if dbName == "" {
		dbName = "garda_inflasi_analytics"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := database.MongoConfig{
		URI:    mongoURI,
		DBName: dbName,
	}

	client, err := database.NewMongo(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	log.Printf("Connected to MongoDB at %s, database: %s\n", mongoURI, dbName)

	if err := seeder.SeedAll(ctx, client, dbName); err != nil {
		log.Fatalf("Seed failed: %v", err)
	}
}
