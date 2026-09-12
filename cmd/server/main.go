package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	appHTTP "github.com/yusqohid/go-htmx-ecommerce/internal/http"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
	"github.com/yusqohid/go-htmx-ecommerce/web/templates"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	log.Printf("Starting Sellora (%s mode)...", cfg.AppEnv)

	// 2. Initialize database connection
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to database (%v). Server will start with degraded state.", err)
	} else {
		defer db.Close()
		log.Println("Database connection established.")
	}

	// 3. Initialize view template renderer
	viewRenderer := view.New(templates.FS, cfg.IsProduction())

	// 4. Initialize services and handlers
	var authService *auth.Service
	var authHandler *auth.Handler

	if db != nil {
		userRepo := auth.NewUserRepository(db.DB)
		sessionRepo := auth.NewSessionRepository(db.DB)
		authService = auth.NewService(userRepo, sessionRepo)
		authHandler = auth.NewHandler(authService, viewRenderer, cfg.IsProduction())
	}

	// 5. Initialize router
	router := appHTTP.NewRouter(appHTTP.RouterDeps{
		Config:      cfg,
		DB:          db,
		AuthService: authService,
		AuthHandler: authHandler,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Start HTTP server in a separate goroutine
	go func() {
		log.Printf("Server listening on port %s (URL: %s)", cfg.AppPort, cfg.AppBaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// 7. Graceful shutdown listening to interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped cleanly.")
}
