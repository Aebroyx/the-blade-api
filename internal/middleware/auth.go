package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Aebroyx/the-blade-api/internal/domain/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Auth middleware with Redis caching
func Auth(jwtSecret string, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get access token from cookie
		accessToken, err := c.Cookie("access_token")
		if err != nil {
			// If access token is not found, try to refresh using refresh token
			if _, err := c.Cookie("refresh_token"); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}

			// TODO: Implement token refresh logic
			// For now, just return unauthorized
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token expired"})
			c.Abort()
			return
		}

		// Parse and validate token
		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature"})
			} else if err == jwt.ErrTokenExpired {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			}
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		var user models.Users
		userKey := fmt.Sprintf("user:%d", claims.UserID)

		// Try to get user from Redis first
		if redisClient != nil {
			userData, err := redisClient.Get(context.Background(), userKey).Bytes()
			if err == nil {
				// Cache hit - unmarshal from Redis
				if err := json.Unmarshal(userData, &user); err == nil {
					log.Printf("Auth middleware: user found in Redis cache for ID %d", claims.UserID)
					goto setUserContext
				}
			}
			// If we get here, either Redis is not available or cache miss
			log.Printf("Auth middleware: Redis cache miss for user ID %d, falling back to database", claims.UserID)
		}

		// DEVELOPMENT MODE: Uncomment this block to use database directly
		/*
			// Get user from database
			if err := db.First(&user, claims.UserID).Error; err != nil {
				log.Printf("Auth middleware: user not found in database for ID %d", claims.UserID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
				c.Abort()
				return
			}
		*/

		// PRODUCTION MODE: Get user from database and cache in Redis
		if err := db.First(&user, claims.UserID).Error; err != nil {
			log.Printf("Auth middleware: user not found in database for ID %d", claims.UserID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Cache user data in Redis if client is available
		if redisClient != nil {
			userJSON, err := json.Marshal(user)
			if err == nil {
				// Cache for 1 hour
				err = redisClient.Set(context.Background(), userKey, userJSON, time.Hour).Err()
				if err != nil {
					log.Printf("Auth middleware: failed to cache user in Redis: %v", err)
				} else {
					log.Printf("Auth middleware: cached user in Redis for ID %d", claims.UserID)
				}
			}
		}

	setUserContext:
		// Create user response object
		userResponse := models.RegisterResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
		}

		log.Printf("Auth middleware: setting user in context: %+v", userResponse)

		// Set user in context
		c.Set("user", userResponse)

		c.Next()
	}
}

// AuthWithoutRedis is the original middleware that only uses database
// Use this for development when Redis is not available
func AuthWithoutRedis(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get access token from cookie
		accessToken, err := c.Cookie("access_token")
		if err != nil {
			// If access token is not found, try to refresh using refresh token
			if _, err := c.Cookie("refresh_token"); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}

			// TODO: Implement token refresh logic
			// For now, just return unauthorized
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token expired"})
			c.Abort()
			return
		}

		// Parse and validate token
		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature"})
			} else if err == jwt.ErrTokenExpired {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			}
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Get user from database
		var user models.Users
		if err := db.First(&user, claims.UserID).Error; err != nil {
			log.Printf("Auth middleware: user not found in database for ID %d", claims.UserID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Create user response object
		userResponse := models.RegisterResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
		}

		log.Printf("Auth middleware: setting user in context: %+v", userResponse)

		// Set user in context
		c.Set("user", userResponse)

		c.Next()
	}
}
