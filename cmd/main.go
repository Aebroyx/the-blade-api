package main

import (
	"log"

	"github.com/Aebroyx/the-blade-api/internal/config"
	"github.com/Aebroyx/the-blade-api/internal/database"
	"github.com/Aebroyx/the-blade-api/internal/handlers"
	"github.com/Aebroyx/the-blade-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
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

	// Initialize router
	router := gin.Default()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, cfg)

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
	protected.Use(middleware.Auth(cfg.JWTSecret))
	{
		// Add your protected routes here
		protected.GET("/me", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(200, gin.H{"user_id": userID})
		})
	}

	// Start server
	log.Printf("Server starting on %s", cfg.GetServerAddr())
	if err := router.Run(cfg.GetServerAddr()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
