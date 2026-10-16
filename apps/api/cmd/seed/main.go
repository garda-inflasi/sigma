package main

import (
	"context"
	"log"
	"os"
	"time"

	coldstorage "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations via Goose (external), seed data here
	log.Println("Database connected, running seed...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cs, err := coldstorage.Seed(ctx, db, time.Now())
	if err != nil {
		log.Fatalf("Seed cold storage failed: %v", err)
	}
	log.Printf("cold storage: %d fasilitas, %d lot stok, %d komoditas baru", cs.Fasilitas, cs.Stok, cs.Komoditas)

	log.Println("Seed completed")

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying DB: %v", err)
	}
	defer sqlDB.Close()
}
