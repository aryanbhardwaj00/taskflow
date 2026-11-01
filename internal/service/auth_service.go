package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
)

type AuthService interface {
	Register(ctx context.Context, email, password string) (*model.User, error)
	Login(ctx context.Context, email, password string) (string, *model.User, error)
	ValidateToken(tokenString string) (*UserClaims, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.User, error) // <-- Add this
}

var (
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrPasswordTooWeak    = errors.New("password must be at least 8 characters long")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// UserClaims defines the JWT claims payload structure.
type UserClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// authService is the concrete implementation of AuthService.
type authService struct {
	userRepo repository.UserRepository
	jwtCfg   config.JWTConfig
}

// NewAuthService constructs a new AuthService with repository and JWT config dependency injection.
func NewAuthService(userRepo repository.UserRepository, jwtCfg config.JWTConfig) AuthService {
	return &authService{
		userRepo: userRepo,
		jwtCfg:   jwtCfg,
	}
}

// Register hashes the user password, initializes default fields, and persists the user.
func (s *authService) Register(ctx context.Context, email, password string) (*model.User, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	if cleanEmail == "" {
		return nil, ErrInvalidEmail
	}

	if len(password) < 8 {
		return nil, ErrPasswordTooWeak
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:        cleanEmail,
		PasswordHash: string(hashedPassword),
		Role:         "member",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login verifies credentials and returns a signed JWT access token along with user details.
func (s *authService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))

	// 1. Fetch user by email
	user, err := s.userRepo.GetByEmail(ctx, cleanEmail)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// Return generic ErrInvalidCredentials to prevent username enumeration attacks
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, fmt.Errorf("login failed: %w", err)
	}

	// 2. Compare password against stored bcrypt hash in constant time
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// 3. Generate JWT access token
	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return token, user, nil
}

// generateToken creates a signed HS256 JWT access token.
func (s *authService) generateToken(user *model.User) (string, error) {
	expirationTime := time.Now().Add(time.Duration(s.jwtCfg.ExpirationHours) * time.Hour)

	claims := &UserClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token with HS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token using symmetric secret key
	tokenString, err := token.SignedString([]byte(s.jwtCfg.Secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken parses and verifies the signature and expiration of a JWT string.
func (s *authService) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Crucial security check: Verify the signing algorithm is specifically HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtCfg.Secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetProfile retrieves a user's details by their ID.
func (s *authService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
