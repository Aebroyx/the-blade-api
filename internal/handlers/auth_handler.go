package handlers

import (
	"github.com/Aebroyx/the-blade-api/internal/config"
	"github.com/Aebroyx/the-blade-api/internal/database"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	db     *database.DB
	config *config.Config
}

func NewAuthHandler(db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:     db,
		config: cfg,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	// Add login logic
}

func (h *AuthHandler) Register(c *gin.Context) {
	// Add registration logic
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Add logout logic
}
