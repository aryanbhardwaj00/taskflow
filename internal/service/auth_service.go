package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrPasswordTooWeak = errors.New("password must be at least 8 characters long")
)

// AuthService defines business operations for authentication and user accounts.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*model.User, error)
}

// authService is the concrete implementation of AuthService.
type authService struct {
	userRepo repository.UserRepository
}

// NewAuthService constructs a new AuthService with repository dependency injection.
func NewAuthService(userRepo repository.UserRepository) AuthService {
	return &authService{
		userRepo: userRepo,
	}
}

func (s *authService) Register(ctx context.Context, email, password string) (*model.User, error) {
	// Normalize email to lowercase
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	if cleanEmail == "" {
		return nil, ErrInvalidEmail
	}

	if len(password) < 8 {
		return nil, ErrPasswordTooWeak
	}

	// Hash password using bcrypt (DefaultCost = 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:        cleanEmail,
		PasswordHash: string(hashedPassword),
		Role:         "member", // default role
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
