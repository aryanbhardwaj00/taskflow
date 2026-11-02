package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/blockaryan/taskflow/internal/middleware"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

// RegisterRequest defines the input payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest defines the input payload for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequest defines the input payload for refreshing an access token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest defines the input payload for revoking a refresh token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserResponse defines the safe public representation of a user.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse defines the authentication response with access and refresh tokens.
type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	User         UserResponse `json:"user"`
}

// RefreshTokenResponse defines the response returned after token rotation.
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler constructs a new AuthHandler with service dependency injection.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register processes user registration requests.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicateEmail):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrPasswordTooWeak), errors.Is(err, service.ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}

// Login processes login requests and issues an access token and refresh token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	tokenPair, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}

// Refresh handles token rotation: validates old refresh token and returns a new pair.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	tokenPair, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid, expired, or already-used refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
	})
}

// Logout revokes the refresh token from Redis.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Me retrieves the authenticated user's profile from the context ID.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}
