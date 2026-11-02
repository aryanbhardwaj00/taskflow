//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
)

// createTestUser is a helper that inserts a valid user into the database for foreign key association.
func createTestUser(t *testing.T, db *infra.DB) *model.User {
	t.Helper()
	repo := repository.NewPostgresUserRepository(db)
	user := &model.User{
		Email:        fmt.Sprintf("user-%s@example.com", uuid.NewString()),
		PasswordHash: "hashedpassword",
		Role:         "user",
	}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err, "Failed to create prerequisite user for task test")
	return user
}

func TestPostgresTaskRepository_Create(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresTaskRepository(db)
	ctx := context.Background()
	user := createTestUser(t, db)

	t.Run("Success: creates a task linked to a user", func(t *testing.T) {
		task := &model.Task{
			UserID:      user.ID,
			Title:       "Test Task Title",
			Description: "Test Task Description",
			Status:      model.StatusTodo,
			Priority:    model.PriorityMedium,
		}

		err := repo.Create(ctx, task)
		require.NoError(t, err)

		assert.NotEqual(t, uuid.Nil, task.ID)
		assert.Equal(t, user.ID, task.UserID)
		assert.False(t, task.CreatedAt.IsZero())
		assert.False(t, task.UpdatedAt.IsZero())
	})

	t.Run("Failure: foreign key constraint fails when user does not exist", func(t *testing.T) {
		fakeUserID := uuid.New()
		task := &model.Task{
			UserID:      fakeUserID,
			Title:       "Orphan Task",
			Description: "Should fail foreign key check",
			Status:      model.StatusTodo,
			Priority:    model.PriorityLow,
		}

		err := repo.Create(ctx, task)
		require.Error(t, err, "Expected foreign key violation error from Postgres")
	})
}

func TestPostgresTaskRepository_GetByID_IDOR(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresTaskRepository(db)
	ctx := context.Background()

	userA := createTestUser(t, db)
	userB := createTestUser(t, db)

	taskA := &model.Task{
		UserID:      userA.ID,
		Title:       "User A Private Task",
		Description: "Confidential details",
		Status:      model.StatusTodo,
		Priority:    model.PriorityHigh,
	}
	err := repo.Create(ctx, taskA)
	require.NoError(t, err)

	t.Run("Success: owner can retrieve their task", func(t *testing.T) {
		found, err := repo.GetByID(ctx, taskA.ID, userA.ID)
		require.NoError(t, err)
		assert.Equal(t, taskA.ID, found.ID)
		assert.Equal(t, "User A Private Task", found.Title)
	})

	t.Run("Security: other user receives ErrTaskNotFound (IDOR protection)", func(t *testing.T) {
		// User B attempts to access User A's task using taskA.ID
		found, err := repo.GetByID(ctx, taskA.ID, userB.ID)
		assert.Nil(t, found)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	})
}

func TestPostgresTaskRepository_ListByUserID(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresTaskRepository(db)
	ctx := context.Background()
	user := createTestUser(t, db)

	// Seed 3 tasks: 2 Todo, 1 Done
	task1 := &model.Task{UserID: user.ID, Title: "Task 1", Status: model.StatusTodo, Priority: model.PriorityLow}
	task2 := &model.Task{UserID: user.ID, Title: "Task 2", Status: model.StatusTodo, Priority: model.PriorityMedium}
	task3 := &model.Task{UserID: user.ID, Title: "Task 3", Status: model.StatusDone, Priority: model.PriorityHigh}

	require.NoError(t, repo.Create(ctx, task1))
	require.NoError(t, repo.Create(ctx, task2))
	require.NoError(t, repo.Create(ctx, task3))

	t.Run("List all tasks for user without filter", func(t *testing.T) {
		tasks, err := repo.ListByUserID(ctx, user.ID, nil, 10, 0)
		require.NoError(t, err)
		assert.Len(t, tasks, 3)
	})

	t.Run("List tasks filtered by StatusTodo", func(t *testing.T) {
		status := model.StatusTodo
		tasks, err := repo.ListByUserID(ctx, user.ID, &status, 10, 0)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		for _, task := range tasks {
			assert.Equal(t, model.StatusTodo, task.Status)
		}
	})

	t.Run("Pagination limit and offset", func(t *testing.T) {
		tasks, err := repo.ListByUserID(ctx, user.ID, nil, 1, 0)
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
	})
}

func TestPostgresTaskRepository_UpdateStatus_AndDelete(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	repo := repository.NewPostgresTaskRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db)
	stranger := createTestUser(t, db)

	task := &model.Task{
		UserID:   user.ID,
		Title:    "Lifecycle Task",
		Status:   model.StatusTodo,
		Priority: model.PriorityMedium,
	}
	require.NoError(t, repo.Create(ctx, task))

	t.Run("Failure: stranger cannot update task status", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, task.ID, stranger.ID, model.StatusInProgress)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	})

	t.Run("Success: owner updates task status", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, task.ID, user.ID, model.StatusInProgress)
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, task.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, model.StatusInProgress, updated.Status)
	})

	t.Run("Failure: stranger cannot delete task", func(t *testing.T) {
		err := repo.Delete(ctx, task.ID, stranger.ID)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	})

	t.Run("Success: owner deletes task", func(t *testing.T) {
		err := repo.Delete(ctx, task.ID, user.ID)
		require.NoError(t, err)

		// Confirm task no longer exists
		found, err := repo.GetByID(ctx, task.ID, user.ID)
		assert.Nil(t, found)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	})
}
