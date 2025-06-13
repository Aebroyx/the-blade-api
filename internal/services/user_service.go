package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Aebroyx/the-blade-api/internal/config"
	"github.com/Aebroyx/the-blade-api/internal/domain/models"
	"github.com/Aebroyx/the-blade-api/internal/pagination"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	db          *gorm.DB
	config      *config.Config
	redisClient *redis.Client
}

// UserQueryParams represents the query parameters for user listing
type UserQueryParams struct {
	Page     int    `json:"page" form:"page" binding:"min=1"`
	PageSize int    `json:"pageSize" form:"pageSize" binding:"min=1,max=100"`
	Search   string `json:"search" form:"search"`
	Role     string `json:"role" form:"role"`
	SortBy   string `json:"sortBy" form:"sortBy" binding:"omitempty,oneof=name email role created_at"`
	SortDesc bool   `json:"sortDesc" form:"sortDesc"`
}

// UserListResponse represents the paginated response for user listing
type UserListResponse struct {
	Data       []models.Users `json:"data"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

func NewUserService(db *gorm.DB, config *config.Config, redisClient *redis.Client) *UserService {
	return &UserService{
		db:          db,
		config:      config,
		redisClient: redisClient,
	}
}

// invalidateUserCache removes the user data from Redis cache
func (s *UserService) invalidateUserCache(userID uint) {
	if s.redisClient != nil {
		userKey := fmt.Sprintf("user:%d", userID)
		err := s.redisClient.Del(context.Background(), userKey).Err()
		if err != nil {
			log.Printf("Failed to invalidate user cache for ID %d: %v", userID, err)
		} else {
			log.Printf("Successfully invalidated user cache for ID %d", userID)
		}
	}
}

// Register creates a new user with the provided registration data
func (s *UserService) Register(req *models.RegisterRequest) (*models.RegisterResponse, error) {
	// Check if username already exists
	var existingUser models.Users
	if err := s.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, errors.New("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if email already exists
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create new user
	user := models.Users{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     "user", // Default role
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	// Return user data without password
	return &models.RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Name:     user.Name,
		Role:     user.Role,
	}, nil
}

// Login authenticates a user and returns tokens
func (s *UserService) Login(req *models.LoginRequest) (*models.LoginResponse, error) {
	// Find user by username
	var user models.Users
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	// Generate tokens
	accessToken, accessExp, err := s.generateToken(user, s.config.JWTExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, _, err := s.generateToken(user, 24*7*time.Hour) // 7 days
	if err != nil {
		return nil, err
	}

	// Create response
	return &models.LoginResponse{
		User: models.RegisterResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
		},
		Token: models.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(accessExp).Seconds()),
		},
	}, nil
}

// generateToken generates a JWT token for the user
func (s *UserService) generateToken(user models.Users, expiry time.Duration) (string, time.Time, error) {
	expirationTime := time.Now().Add(expiry)
	claims := &models.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "the-blade-api",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// GetAllUsers retrieves users with pagination, search, and filters
func (s *UserService) GetAllUsers(params pagination.QueryParams) (*pagination.PaginatedResponse, error) {
	config := pagination.PaginationConfig{
		Model: &models.Users{},
		BaseCondition: map[string]interface{}{
			"is_deleted": false,
		},
		SearchFields: []string{"name", "email", "username"},
		FilterFields: map[string]string{
			"role":       "role",
			"name":       "name",
			"email":      "email",
			"username":   "username",
			"created_at": "created_at",
			"updated_at": "updated_at",
		},
		DateFields: map[string]pagination.DateField{
			"created_at": {
				Start: "created_at",
				End:   "created_at",
			},
			"updated_at": {
				Start: "updated_at",
				End:   "updated_at",
			},
		},
		SortFields: []string{
			"name",
			"email",
			"role",
			"created_at",
			"updated_at",
		},
		DefaultSort:  "created_at",
		DefaultOrder: "DESC",
	}

	paginator := pagination.NewPaginator(s.db)
	return paginator.Paginate(params, config)

	// Pagination Example (with join)
	// GetAllUsers retrieves users with pagination, search, and filters
	// config := pagination.PaginationConfig{
	// 	Model: &models.Users{},
	// 	BaseCondition: map[string]interface{}{
	// 		"is_deleted": false,
	// 	},
	// 	SearchFields: []string{"name", "email", "username"},
	// 	FilterFields: map[string]string{
	// 		"role": "role",
	// 	},
	// 	DateFields: map[string]pagination.DateField{
	// 		"created_at": {
	// 			Start: "created_at",
	// 			End:   "created_at",
	// 		},
	// 		"updated_at": {
	// 			Start: "updated_at",
	// 			End:   "updated_at",
	// 		},
	// 	},
	// 	SortFields: []string{
	// 		"name",
	// 		"email",
	// 		"role",
	// 		"created_at",
	// 		"updated_at",
	// 	},
	// 	DefaultSort:  "created_at",
	// 	DefaultOrder: "DESC",
	// }

	// paginator := pagination.NewPaginator(s.db)
	// return paginator.Paginate(params, config)
}

func (s *UserService) GetUserById(id string) (models.Users, error) {
	var user models.Users
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return models.Users{}, err
	}
	return user, nil
}

// CreateUser creates a new user with the provided data
func (s *UserService) CreateUser(req *models.CreateUserRequest) (*models.CreateUserResponse, error) {
	// Check if username already exists
	var existingUser models.Users
	if err := s.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, errors.New("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if email already exists
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create new user
	user := models.Users{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     req.Role,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	// Return user data without password
	return &models.CreateUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *UserService) UpdateUser(id string, req *models.UpdateUserRequest) (*models.Users, error) {
	var user models.Users
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	// Update user fields
	user.Username = req.Username
	user.Email = req.Email
	user.Name = req.Name
	user.Role = req.Role

	// Only update password if provided
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}

	// Update user
	if err := s.db.Model(&user).Updates(&user).Error; err != nil {
		return nil, err
	}

	// Invalidate user cache after update
	s.invalidateUserCache(user.ID)

	return &user, nil
}

func (s *UserService) DeleteUser(id string) (*models.Users, error) {
	var user models.Users
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	if err := s.db.Delete(&user).Error; err != nil {
		return nil, err
	}

	// Invalidate user cache after deletion
	s.invalidateUserCache(user.ID)

	return &user, nil
}

func (s *UserService) SoftDeleteUser(id string) (*models.Users, error) {
	var user models.Users
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&user).Update("is_deleted", true).Error; err != nil {
		return nil, err
	}

	// Invalidate user cache after soft deletion
	s.invalidateUserCache(user.ID)

	return &user, nil
}
