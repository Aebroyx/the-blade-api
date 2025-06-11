package common

import "github.com/gin-gonic/gin"

// Response represents a standardized API response
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(message string, code string, details any) ErrorResponse {
	return ErrorResponse{
		Status:  "error",
		Message: message,
		Code:    code,
		Details: details,
	}
}

// SendError sends an error response
func SendError(c *gin.Context, status int, message string, code string, details any) {
	c.JSON(status, NewErrorResponse(message, code, details))
}

// SendSuccess sends a success response
func SendSuccess(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Response{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// Common error codes
const (
	CodeInvalidRequest  = "INVALID_REQUEST"
	CodeValidationError = "VALIDATION_ERROR"
	CodeUsernameExists  = "USERNAME_EXISTS"
	CodeEmailExists     = "EMAIL_EXISTS"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeBadRequest      = "BAD_REQUEST"
	CodeConflict        = "CONFLICT"
)

// Common error responses
var (
	ErrInvalidRequest = func(details any) ErrorResponse {
		return NewErrorResponse("Invalid request body", CodeInvalidRequest, details)
	}
	ErrValidation = func(details any) ErrorResponse {
		return NewErrorResponse("Validation failed", CodeValidationError, details)
	}
	ErrUsernameExists = ErrorResponse{
		Status:  "error",
		Message: "Username already exists",
		Code:    CodeUsernameExists,
		Details: map[string]string{
			"field":  "username",
			"reason": "already_taken",
		},
	}
	ErrEmailExists = ErrorResponse{
		Status:  "error",
		Message: "Email already exists",
		Code:    CodeEmailExists,
		Details: map[string]string{
			"field":  "email",
			"reason": "already_taken",
		},
	}
	ErrInternal = ErrorResponse{
		Status:  "error",
		Message: "Internal server error",
		Code:    CodeInternalError,
	}
)
