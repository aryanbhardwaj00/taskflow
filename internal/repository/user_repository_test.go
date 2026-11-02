//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
)

// setupTestDB connects to the live Docker PostgreSQL database and provides a cleanup function.
func setupTestDB(t *testing.T) (*infra.DB, func()) {
	t.Helper()

	cfg := config.DBConfig{
		Host:     "localhost",
		Port:     "5431",
		User:     "postgres",
		Password: "postgres123",
		Name:     "taskflow_db",
		SSLMode:  "disable",
	}

	db, err := infra.NewPostgresDB(cfg)
	require.NoError(t, err, "Failed to connect to integration test database")

	// Cleanup truncates the tables so each test starts with a clean slate
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := db.Pool.Exec(ctx, "TRUNCATE TABLE tasks, users CASCADE;")
		assert.NoError(t, err, "Failed to truncate tables during test cleanup")
		db.Close()
	}

	return db, cleanup
}

func TestPostgresUserRepository_Create(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	t.Run("Success: creates a user and populates auto-generated fields", func(t *testing.T) {
		user := &model.User{
			Email:        fmt.Sprintf("test-%s@example.com", uuid.NewString()),
			PasswordHash: "hashedpassword123",
			Role:         "user",
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		// Assert that Postgres populated ID and timestamps
		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
	})

	t.Run("Failure: duplicate email returns ErrDuplicateEmail", func(t *testing.T) {
		email := fmt.Sprintf("duplicate-%s@example.com", uuid.NewString())

		user1 := &model.User{
			Email:        email,
			PasswordHash: "hashed1",
			Role:         "user",
		}
		err := repo.Create(ctx, user1)
		require.NoError(t, err)

		user2 := &model.User{
			Email:        email,
			PasswordHash: "hashed2",
			Role:         "user",
		}
		err = repo.Create(ctx, user2)
		require.ErrorIs(t, err, repository.ErrDuplicateEmail)
	})
}

func TestPostgresUserRepository_GetByEmail(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	t.Run("Success: finds existing user by email", func(t *testing.T) {
		email := fmt.Sprintf("findme-%s@example.com", uuid.NewString())
		user := &model.User{
			Email:        email,
			PasswordHash: "somehash",
			Role:         "admin",
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		found, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, email, found.Email)
		assert.Equal(t, "admin", found.Role)
	})

	t.Run("Failure: non-existent email returns ErrUserNotFound", func(t *testing.T) {
		found, err := repo.GetByEmail(ctx, "nonexistent@example.com")
		assert.Nil(t, found)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})
}

func TestPostgresUserRepository_GetByID(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	t.Run("Success: finds existing user by ID", func(t *testing.T) {
		user := &model.User{
			Email:        fmt.Sprintf("byid-%s@example.com", uuid.NewString()),
			PasswordHash: "somehash",
			Role:         "user",
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("Failure: non-existent ID returns ErrUserNotFound", func(t *testing.T) {
		randomID := uuid.New()
		found, err := repo.GetByID(ctx, randomID)
		assert.Nil(t, found)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})
}
