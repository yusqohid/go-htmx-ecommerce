package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	"github.com/yusqohid/go-htmx-ecommerce/migrations"
)

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/sellora?sslmode=disable"
	}

	db, err := database.New(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Starting database migrations...")
	if err := database.MigrateUp(ctx, db.DB, migrations.FS); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("All migrations applied successfully!")
}
