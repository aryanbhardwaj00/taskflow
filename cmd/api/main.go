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

	"github.com/gin-gonic/gin"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/handler"
	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

func main() {
	// 1. Load and validate environment configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v\n", err)
	}

	// 2. Initialize PostgreSQL connection pool
	db, err := infra.NewPostgresDB(cfg.DB)
	if err != nil {
		log.Fatalf("Failed to establish database connection: %v\n", err)
	}
	defer db.Close()

	// 3. Initialize Repositories
	userRepo := repository.NewPostgresUserRepository(db)

	// 4. Initialize Services
	authService := service.NewAuthService(userRepo)

	// 5. Initialize Handlers
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)

	// 6. Set Gin mode based on environment
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 7. Initialize Gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 8. Register Routes
	router.GET("/health", healthHandler.HealthCheck)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthHandler.HealthCheck)

		// Auth routes group
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
		}
	}

	// 9. Configure http.Server using values from config
	serverAddr := fmt.Sprintf(":%s", cfg.App.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	// 10. Start server in a background goroutine
	go func() {
		log.Printf("Server listening on port %s in %s mode", cfg.App.Port, cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server startup failed: %v\n", err)
		}
	}()

	// 11. Graceful shutdown listening for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	log.Printf("Received signal %s. Initiating graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	}

	log.Println("Server exiting successfully.")
}
