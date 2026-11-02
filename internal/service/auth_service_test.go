package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

// MockUserRepository implements repository.UserRepository for testing.
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

// TestRegister tests the Register business logic using table-driven tests.
func TestRegister(t *testing.T) {
	ctx := context.Background()
	jwtCfg := config.JWTConfig{Secret: "test-secret-key-32-bytes-long!!", ExpirationHours: 24}

	testCases := []struct {
		name          string
		email         string
		password      string
		setupMock     func(mockRepo *MockUserRepository)
		expectedErr   error
		checkResponse func(t *testing.T, user *model.User)
	}{
		{
			name:     "Success: valid registration",
			email:    "Ryan@Example.COM", // tests case normalization
			password: "ValidPassword123",
			setupMock: func(mockRepo *MockUserRepository) {
				mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					// Verify email is lowercased and password was actually hashed
					return u.Email == "ryan@example.com" &&
						bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("ValidPassword123")) == nil
				})).Return(nil)
			},
			expectedErr: nil,
			checkResponse: func(t *testing.T, user *model.User) {
				assert.NotNil(t, user)
				assert.Equal(t, "ryan@example.com", user.Email)
				assert.Equal(t, "member", user.Role)
			},
		},
		{
			name:        "Failure: empty email",
			email:       "   ",
			password:    "ValidPassword123",
			setupMock:   func(mockRepo *MockUserRepository) {}, 
			expectedErr: service.ErrInvalidEmail,
			checkResponse: func(t *testing.T, user *model.User) {
				assert.Nil(t, user)
			},
		},
		{
			name:        "Failure: password too short (< 8 chars)",
			email:       "ryan@example.com",
			password:    "short",
			setupMock:   func(mockRepo *MockUserRepository) {},
			expectedErr: service.ErrPasswordTooWeak,
			checkResponse: func(t *testing.T, user *model.User) {
				assert.Nil(t, user)
			},
		},
		{
			name:     "Failure: duplicate email",
			email:    "ryan@example.com",
			password: "ValidPassword123",
			setupMock: func(mockRepo *MockUserRepository) {
				mockRepo.On("Create", mock.Anything, mock.Anything).Return(repository.ErrDuplicateEmail)
			},
			expectedErr: repository.ErrDuplicateEmail,
			checkResponse: func(t *testing.T, user *model.User) {
				assert.Nil(t, user)
			},
		},
		{
			name:     "Failure: unexpected repository error",
			email:    "ryan@example.com",
			password: "ValidPassword123",
			setupMock: func(mockRepo *MockUserRepository) {
				mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database connection refused"))
			},
			expectedErr: errors.New("database connection refused"),
			checkResponse: func(t *testing.T, user *model.User) {
				assert.Nil(t, user)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepository)
			tc.setupMock(mockRepo)

			// Instantiate service with mock repository (nil Redis since Register doesn't use Redis)
			authSvc := service.NewAuthService(mockRepo, jwtCfg, nil)

			user, err := authSvc.Register(ctx, tc.email, tc.password)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}

			tc.checkResponse(t, user)

			// Crucial: Assert that all expected mock calls actually happened
			mockRepo.AssertExpectations(t)
		})
	}
}
