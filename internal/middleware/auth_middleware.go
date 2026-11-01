package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/blockaryan/taskflow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	ContextUserIDKey    = "userID"
	ContextUserEmailKey = "userEmail"
	ContextUserRoleKey  = "userRole"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrUserNotInContext = errors.New("user identity not found in context")
)

func AuthMiddleware(authserive service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header must be in 'Bearer <token>' format",
			})
			return
		}
		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token is missing",
			})
			return
		}

		claims, err := authserive.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}
		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUserEmailKey, claims.Email)
		c.Set(ContextUserRoleKey, claims.Role)

		// 5. Proceed to the next handler
		c.Next()
	}
}

// GetUserID extracts the authenticated user's UUID from the gin.Context.
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get(ContextUserIDKey)
	if !exists {
		return uuid.Nil, ErrUserNotInContext
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user ID type in context")
	}

	return userID, nil
}

// GetUserRole extracts the authenticated user's role from the gin.Context.
func GetUserRole(c *gin.Context) (string, error) {
	val, exists := c.Get(ContextUserRoleKey)
	if !exists {
		return "", ErrUserNotInContext
	}

	role, ok := val.(string)
	if !ok {
		return "", errors.New("invalid user role type in context")
	}

	return role, nil
}

// RequireRoles restricts endpoint access to users possessing one of the specified roles.
func RequireRoles(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, err := GetUserRole(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Check if user's role matches any allowed role
		isAllowed := false
		for _, allowed := range allowedRoles {
			if strings.EqualFold(role, allowed) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access forbidden: insufficient role permissions",
			})
			return
		}

		c.Next()
	}
}
