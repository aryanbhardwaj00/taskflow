package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/blockaryan/taskflow/internal/middleware"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/service"
)

// MockAuthService implements service.AuthService for testing.
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
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func TestAuthMiddleware(t *testing.T) {
	// Set Gin to test mode to suppress debug logs during testing
	gin.SetMode(gin.TestMode)

	testUserID := uuid.New()

	testCases := []struct {
		name           string
		authHeader     string
		setupMock      func(mockAuth *MockAuthService)
		expectedStatus int
		expectInCtx    bool
	}{
		{
			name:           "Failure: missing authorization header",
			authHeader:     "",
			setupMock:      func(mockAuth *MockAuthService) {}, // Mock should not be called
			expectedStatus: http.StatusUnauthorized,
			expectInCtx:    false,
		},
		{
			name:           "Failure: malformed header (missing Bearer prefix)",
			authHeader:     "Basic dXNlcjpwYXNz",
			setupMock:      func(mockAuth *MockAuthService) {},
			expectedStatus: http.StatusUnauthorized,
			expectInCtx:    false,
		},
		{
			name:       "Failure: invalid or expired token",
			authHeader: "Bearer invalid-or-expired-token",
			setupMock: func(mockAuth *MockAuthService) {
				mockAuth.On("ValidateToken", "invalid-or-expired-token").Return(nil, errors.New("token expired"))
			},
			expectedStatus: http.StatusUnauthorized,
			expectInCtx:    false,
		},
		{
			name:       "Success: valid token passes and populates context",
			authHeader: "Bearer valid-jwt-token",
			setupMock: func(mockAuth *MockAuthService) {
				mockAuth.On("ValidateToken", "valid-jwt-token").Return(&service.UserClaims{
					UserID: testUserID,
					Email:  "ryan@example.com",
					Role:   "member",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectInCtx:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAuth := new(MockAuthService)
			tc.setupMock(mockAuth)

			// 1. Create a test Gin engine with our middleware
			router := gin.New()
			router.Use(middleware.AuthMiddleware(mockAuth))

			// 2. Add a dummy downstream route to verify if the request gets through
			var capturedUserID uuid.UUID
			router.GET("/protected-test", func(c *gin.Context) {
				capturedUserID, _ = middleware.GetUserID(c)
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			// 3. Create test HTTP request & recorder
			req := httptest.NewRequest(http.MethodGet, "/protected-test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			w := httptest.NewRecorder()

			// 4. Dispatch request into router
			router.ServeHTTP(w, req)

			// 5. Assertions
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectInCtx {
				assert.Equal(t, testUserID, capturedUserID)
			}

			mockAuth.AssertExpectations(t)
		})
	}
}
