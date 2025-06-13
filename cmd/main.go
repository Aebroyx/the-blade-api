package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Aebroyx/the-blade-api/internal/config"
	"github.com/Aebroyx/the-blade-api/internal/database"
	"github.com/Aebroyx/the-blade-api/internal/handlers"
	"github.com/Aebroyx/the-blade-api/internal/middleware"
	"github.com/Aebroyx/the-blade-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create background context
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize database
	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Redis client
	var redisClient *redis.Client
	if cfg.UseRedis {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		// Test Redis connection
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v. Running without Redis caching.", err)
			redisClient = nil
		} else {
			log.Printf("Successfully connected to Redis at %s:%s", cfg.RedisHost, cfg.RedisPort)
		}
	}

	// Initialize services
	userService := services.NewUserService(db.DB, cfg, redisClient)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService)
	userHandler := handlers.NewUserHandler(userService)

	// Initialize router
	router := gin.New() // Use gin.New() instead of gin.Default() to avoid default middleware

	// Add logger middleware
	router.Use(gin.Logger())

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		// Log incoming request
		log.Printf("Incoming request: %s %s", c.Request.Method, c.Request.URL.Path)

		// Get allowed origins from config
		allowedOrigin := cfg.CORSAllowedOrigins
		if allowedOrigin == "" {
			allowedOrigin = "http://localhost:3001" // fallback
		}

		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight
		if c.Request.Method == "OPTIONS" {
			log.Printf("Handling OPTIONS request for: %s", c.Request.URL.Path)
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Public routes
	public := router.Group("/api")
	{
		// Auth routes
		auth := public.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}
	}

	// Protected routes
	protected := router.Group("/api")

	// Use appropriate auth middleware based on Redis availability
	if redisClient != nil {
		protected.Use(middleware.Auth(cfg.JWTSecret, db.DB, redisClient))
		log.Println("Using Redis-enabled auth middleware")
	} else {
		protected.Use(middleware.AuthWithoutRedis(cfg.JWTSecret, db.DB))
		log.Println("Using database-only auth middleware")
	}

	{
		// AUTH ROUTES
		protected.GET("/me", authHandler.GetMe)
		protected.POST("/auth/logout", authHandler.Logout)
		// USER ROUTES
		protected.GET("/users", userHandler.GetAllUsers)
		user := protected.Group("/user")
		{
			user.GET("/:id", userHandler.GetUserById)
			user.POST("/create", userHandler.CreateUser)
			user.PUT("/:id", userHandler.UpdateUser)
			user.DELETE("/:id", userHandler.DeleteUser)
			user.PUT("/:id/soft-delete", userHandler.SoftDeleteUser)
		}
	}

	// Start server
	log.Printf("Server starting on %s", cfg.GetServerAddr())
	if err := router.Run(cfg.GetServerAddr()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
