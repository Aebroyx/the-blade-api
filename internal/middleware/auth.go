package middleware

import (
	"github.com/gin-gonic/gin"
)

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add your JWT validation logic here
		c.Next()
	}
}
