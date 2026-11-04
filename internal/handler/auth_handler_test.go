package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/blockaryan/taskflow/internal/handler"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

// MockAuthService implements service.AuthService for testing handlers.
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(ctx context.Context, email, password string) (*model.User, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*service.TokenPair, *model.User, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*service.TokenPair), args.Get(1).(*model.User), args.Error(2)
}

func (m *MockAuthService) ValidateToken(tokenString string) (*service.UserClaims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.UserClaims), args.Error(1)
}

func (m *MockAuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func setupAuthRouter(mockAuth *MockAuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewAuthHandler(mockAuth)

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.Logout)
	}
	return r
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("Success: 201 Created", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		expectedUser := &model.User{
			ID:        uuid.New(),
			Email:     "ryan@example.com",
			Role:      "member",
			CreatedAt: time.Now(),
		}

		mockAuth.On("Register", mock.Anything, "ryan@example.com", "Password123!").
			Return(expectedUser, nil)

		payload := handler.RegisterRequest{
			Email:    "ryan@example.com",
			Password: "Password123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockAuth.AssertExpectations(t)
	})

	t.Run("Failure: 409 Conflict on duplicate email", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		mockAuth.On("Register", mock.Anything, "ryan@example.com", "Password123!").
			Return(nil, repository.ErrDuplicateEmail)

		payload := handler.RegisterRequest{
			Email:    "ryan@example.com",
			Password: "Password123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		mockAuth.AssertExpectations(t)
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("Success: 200 OK with token pair", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		user := &model.User{
			ID:        uuid.New(),
			Email:     "ryan@example.com",
			Role:      "member",
			CreatedAt: time.Now(),
		}
		tokens := &service.TokenPair{
			AccessToken:  "valid-jwt-token",
			RefreshToken: "valid-refresh-token",
		}

		mockAuth.On("Login", mock.Anything, "ryan@example.com", "Password123!").
			Return(tokens, user, nil)

		payload := handler.LoginRequest{
			Email:    "ryan@example.com",
			Password: "Password123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockAuth.AssertExpectations(t)
	})

	t.Run("Failure: 401 Unauthorized on invalid credentials", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		mockAuth.On("Login", mock.Anything, "ryan@example.com", "WrongPassword!").
			Return(nil, nil, service.ErrInvalidCredentials)

		payload := handler.LoginRequest{
			Email:    "ryan@example.com",
			Password: "WrongPassword!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockAuth.AssertExpectations(t)
	})
}

func TestAuthHandler_RefreshToken(t *testing.T) {
	t.Run("Success: 200 OK with rotated token pair", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		tokens := &service.TokenPair{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		}

		mockAuth.On("RefreshToken", mock.Anything, "valid-refresh-token").
			Return(tokens, nil)

		payload := handler.RefreshTokenRequest{RefreshToken: "valid-refresh-token"}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockAuth.AssertExpectations(t)
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Run("Success: 200 OK idempotent logout", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		router := setupAuthRouter(mockAuth)

		mockAuth.On("Logout", mock.Anything, "some-refresh-token").Return(nil)

		payload := handler.LogoutRequest{RefreshToken: "some-refresh-token"}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockAuth.AssertExpectations(t)
	})
}
