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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/handler"
	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/middleware"
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

	// 3. Initialize Redis client
	redisClient, err := infra.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to establish Redis connection: %v\n", err)
	}
	defer redisClient.Close()

	// 4. Initialize Repositories (Data Access Layer)
	userRepo := repository.NewPostgresUserRepository(db)
	taskRepo := repository.NewPostgresTaskRepository(db)

	// 5. Initialize Services (Business Logic Layer)
	authService := service.NewAuthService(userRepo, cfg.JWT, redisClient)
	taskService := service.NewTaskService(taskRepo, redisClient)

	// 6. Initialize Handlers (Presentation Layer)
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	taskHandler := handler.NewTaskHandler(taskService)

	// 7. Set Gin mode based on environment
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 8. Initialize Gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.PrometheusMetrics()) // Intercept all requests for Prometheus metrics

	// 9. Register Routes
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler())) // Expose Prometheus metrics endpoint

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthHandler.HealthCheck)

		// Verification route to test Redis in action
		v1.GET("/redis-test", func(c *gin.Context) {
			ctx := c.Request.Context()
			key := "test:greeting"

			// Set a test key with 60-second TTL
			if err := redisClient.Set(ctx, key, "Hello from Redis!", 60*time.Second); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set Redis key", "details": err.Error()})
				return
			}

			// Read back the key
			val, err := redisClient.Get(ctx, key)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Redis key", "details": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Redis is operational",
				"key":     key,
				"value":   val,
				"ttl":     "60s",
			})
		})

		// Public Auth routes
		auth := v1.Group("/auth")
		auth.Use(middleware.RateLimiter(redisClient, 5, 1*time.Minute))
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/logout", authHandler.Logout)
		}

		// Protected routes (Require valid JWT)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(authService))
		protected.Use(middleware.RateLimiter(redisClient, 10, 1*time.Minute))
		{
			protected.GET("/auth/me", authHandler.Me)

			// Task Management routes
			tasks := protected.Group("/tasks")
			{
				tasks.POST("", taskHandler.Create)
				tasks.GET("", taskHandler.List)
				tasks.GET("/:id", taskHandler.Get)
				tasks.PATCH("/:id/status", taskHandler.UpdateStatus)
				tasks.DELETE("/:id", taskHandler.Delete)
			}

			// Admin-only routes
			admin := protected.Group("/admin")
			admin.Use(middleware.RequireRoles("admin"))
			{
				admin.GET("/stats", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{
						"status":  "success",
						"message": "Welcome to the Admin Portal",
					})
				})
			}
		}
	}

	// 10. Configure http.Server using values from config
	serverAddr := fmt.Sprintf(":%s", cfg.App.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	// 11. Start server in a background goroutine
	go func() {
		log.Printf("Server listening on port %s in %s mode", cfg.App.Port, cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server startup failed: %v\n", err)
		}
	}()

	// 12. Graceful shutdown listening for OS signals
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
