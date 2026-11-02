package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour // 7 days
)

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

// TokenPair contains both access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// AuthService defines business operations for authentication and user accounts.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*model.User, error)
	Login(ctx context.Context, email, password string) (*TokenPair, *model.User, error)
	ValidateToken(tokenString string) (*UserClaims, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

// authService is the concrete implementation of AuthService.
type authService struct {
	userRepo repository.UserRepository
	jwtCfg   config.JWTConfig
	redis    *infra.RedisClient
}

// NewAuthService constructs a new AuthService with repository, config, and Redis injection.
func NewAuthService(
	userRepo repository.UserRepository,
	jwtCfg config.JWTConfig,
	redis *infra.RedisClient,
) AuthService {
	return &authService{
		userRepo: userRepo,
		jwtCfg:   jwtCfg,
		redis:    redis,
	}
}

// refreshTokenKey generates the Redis key for storing a refresh token.
func refreshTokenKey(token string) string {
	return fmt.Sprintf("refreshtoken:%s", token)
}

// generateSecureRandomString creates a cryptographically secure 32-byte hex string.
func generateSecureRandomString() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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

// Login verifies credentials and returns a new Access Token and Refresh Token.
func (s *authService) Login(ctx context.Context, email, password string) (*TokenPair, *model.User, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.GetByEmail(ctx, cleanEmail)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("login failed: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// 1. Generate short-lived Access Token (15 mins)
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// 2. Generate long-lived opaque Refresh Token (7 days)
	refreshToken, err := generateSecureRandomString()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 3. Store refresh token in Redis
	if err := s.redis.Set(ctx, refreshTokenKey(refreshToken), user.ID.String(), refreshTokenTTL); err != nil {
		return nil, nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, user, nil
}

// RefreshToken validates a refresh token, performs rotation, and returns a new pair.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	key := refreshTokenKey(refreshToken)

	// 1. Check if refresh token exists in Redis
	userIDStr, err := s.redis.Get(ctx, key)
	if err != nil {
		// If key does not exist: token is expired, revoked, or already consumed (theft attempt)
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// 2. Fetch user to verify account is still active and retrieve latest role
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// 3. TOKEN ROTATION: Invalidate the used refresh token immediately
	_ = s.redis.Del(ctx, key)

	// 4. Generate new Access Token (15 min)
	newAccessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// 5. Generate new Refresh Token (7 days)
	newRefreshToken, err := generateSecureRandomString()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 6. Save new refresh token in Redis
	if err := s.redis.Set(ctx, refreshTokenKey(newRefreshToken), user.ID.String(), refreshTokenTTL); err != nil {
		return nil, fmt.Errorf("failed to store new refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout revokes the refresh token from Redis.
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	return s.redis.Del(ctx, refreshTokenKey(refreshToken))
}

// generateAccessToken creates a short-lived signed HS256 JWT access token.
func (s *authService) generateAccessToken(user *model.User) (string, error) {
	expirationTime := time.Now().Add(accessTokenTTL)

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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtCfg.Secret))
}

// ValidateToken parses and verifies the signature and expiration of an access token.
func (s *authService) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
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
