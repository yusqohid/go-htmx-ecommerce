package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	userRepo := auth.NewUserRepository(db.DB)
	sessionRepo := auth.NewSessionRepository(db.DB)
	authService := auth.NewService(userRepo, sessionRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminEmail := "admin@sellora.local"
	adminPassword := "AdminPassword123!"
	adminName := "Store Administrator"

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
