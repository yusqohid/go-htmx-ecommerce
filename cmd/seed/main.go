package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
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

	userRepo := auth.NewUserRepository(db.DB)
	sessionRepo := auth.NewSessionRepository(db.DB)
	authService := auth.NewService(userRepo, sessionRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminEmail := os.Getenv("SEED_ADMIN_EMAIL")
	if strings.TrimSpace(adminEmail) == "" {
		adminEmail = "admin@sellora.local"
	}

	adminPassword := os.Getenv("SEED_ADMIN_PASSWORD")
	if strings.TrimSpace(adminPassword) == "" {
		adminPassword = "AdminPassword123!"
	}

	adminName := os.Getenv("SEED_ADMIN_NAME")
	if strings.TrimSpace(adminName) == "" {
		adminName = "Store Administrator"
	}
	log.Printf("Checking for default admin account (%s)...", adminEmail)
	admin, err := authService.CreateAdmin(ctx, adminName, adminEmail, adminPassword)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			log.Printf("Admin account (%s) already exists. No action taken.", adminEmail)
			return
		}
		log.Fatalf("Failed to seed admin account: %v", err)
	}

	log.Println("======================================================")
	log.Printf("Default Admin created successfully (ID: %d)", admin.ID)
	log.Printf("Email:    %s", admin.Email)
	log.Printf("Password: %s", adminPassword)
	log.Printf("Role:     %s", admin.Role)
	log.Println("Please change this password after your first login!")
	log.Println("======================================================")
}
