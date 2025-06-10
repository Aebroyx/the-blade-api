package handlers

import (
	"net/http"

	"github.com/Aebroyx/the-blade-api/internal/common"
	"github.com/Aebroyx/the-blade-api/internal/domain/models"
	"github.com/Aebroyx/the-blade-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userService *services.UserService
	validate    *validator.Validate
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
		validate:    validator.New(),
	}
}

func (h *UserHandler) GetAllUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	common.SendSuccess(c, http.StatusOK, "Users fetched successfully", users)
}

func (h *UserHandler) GetUserById(c *gin.Context) {
	user, err := h.userService.GetUserById(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	common.SendSuccess(c, http.StatusOK, "User fetched successfully", user)
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.SendError(c, http.StatusBadRequest, "Invalid request body", common.CodeInvalidRequest, err.Error())
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		common.SendError(c, http.StatusBadRequest, "Validation failed", common.CodeValidationError, err.Error())
		return
	}

	// Create user
	user, err := h.userService.CreateUser(&req)
	if err != nil {
		switch err.Error() {
		case "username already exists":
			common.SendError(c, http.StatusConflict, "Username already exists", common.CodeUsernameExists, nil)
		case "email already exists":
			common.SendError(c, http.StatusConflict, "Email already exists", common.CodeEmailExists, nil)
		default:
			common.SendError(c, http.StatusInternalServerError, "Internal server error", common.CodeInternalError, nil)
		}
		return
	}

	common.SendSuccess(c, http.StatusCreated, "User created successfully", user)
}
