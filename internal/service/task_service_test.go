package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

// MockTaskRepository implements repository.TaskRepository for unit testing.
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *model.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Task, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockTaskRepository) ListByUserID(ctx context.Context, userID uuid.UUID, status *model.TaskStatus, limit, offset int) ([]model.Task, error) {
	args := m.Called(ctx, userID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Task), args.Error(1)
}

func (m *MockTaskRepository) UpdateStatus(ctx context.Context, id, userID uuid.UUID, status model.TaskStatus) error {
	args := m.Called(ctx, id, userID, status)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

// TestCreateTask verifies business rules and defaults when creating a task.
func TestCreateTask(t *testing.T) {
	ctx := context.Background()
	testUserID := uuid.New()

	testCases := []struct {
		name          string
		title         string
		description   string
		priority      model.TaskPriority
		setupMock     func(m *MockTaskRepository)
		expectedErr   error
		checkResponse func(t *testing.T, task *model.Task)
	}{
		{
			name:        "Success: valid task creation with default medium priority",
			title:       "Buy groceries",
			description: "Milk, eggs, coffee",
			priority:    "", // should default to medium
			setupMock: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(task *model.Task) bool {
					return task.Title == "Buy groceries" &&
						task.Description == "Milk, eggs, coffee" &&
						task.Priority == model.PriorityMedium &&
						task.Status == model.StatusTodo &&
						task.UserID == testUserID
				})).Return(nil)
			},
			expectedErr: nil,
			checkResponse: func(t *testing.T, task *model.Task) {
				assert.NotNil(t, task)
				assert.Equal(t, "Buy groceries", task.Title)
				assert.Equal(t, model.PriorityMedium, task.Priority)
				assert.Equal(t, model.StatusTodo, task.Status)
			},
		},
		{
			name:        "Success: valid task creation with explicit high priority",
			title:       "Deploy to production",
			description: "Run migrations and boot containers",
			priority:    model.PriorityHigh,
			setupMock: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(task *model.Task) bool {
					return task.Priority == model.PriorityHigh
				})).Return(nil)
			},
			expectedErr: nil,
			checkResponse: func(t *testing.T, task *model.Task) {
				assert.NotNil(t, task)
				assert.Equal(t, model.PriorityHigh, task.Priority)
			},
		},
		{
			name:        "Failure: empty title",
			title:       "   ",
			description: "Some description",
			priority:    model.PriorityMedium,
			setupMock:   func(m *MockTaskRepository) {},
			expectedErr: service.ErrInvalidTaskTitle,
			checkResponse: func(t *testing.T, task *model.Task) {
				assert.Nil(t, task)
			},
		},
		{
			name:        "Failure: invalid priority string",
			title:       "Urgent bug fix",
			description: "Fix memory leak",
			priority:    "urgent",
			setupMock:   func(m *MockTaskRepository) {},
			expectedErr: service.ErrInvalidTaskPriority,
			checkResponse: func(t *testing.T, task *model.Task) {
				assert.Nil(t, task)
			},
		},
		{
			name:        "Failure: repository database error",
			title:       "Test task",
			description: "Database is down",
			priority:    model.PriorityLow,
			setupMock: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.Anything).Return(errors.New("db connection lost"))
			},
			expectedErr: errors.New("db connection lost"),
			checkResponse: func(t *testing.T, task *model.Task) {
				assert.Nil(t, task)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockTaskRepository)
			tc.setupMock(mockRepo)

			taskSvc := service.NewTaskService(mockRepo, nil)
			task, err := taskSvc.CreateTask(ctx, testUserID, tc.title, tc.description, tc.priority)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}

			tc.checkResponse(t, task)
			mockRepo.AssertExpectations(t)
		})
	}
}

// TestGetTask verifies retrieval and repository fallback logic.
func TestGetTask(t *testing.T) {
	ctx := context.Background()
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: task fetched from database", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		expectedTask := &model.Task{
			ID:          testTaskID,
			UserID:      testUserID,
			Title:       "Existing Task",
			Status:      model.StatusTodo,
			Priority:    model.PriorityMedium,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		mockRepo.On("GetByID", mock.Anything, testTaskID, testUserID).Return(expectedTask, nil)

		taskSvc := service.NewTaskService(mockRepo, nil)
		task, err := taskSvc.GetTask(ctx, testTaskID, testUserID)

		assert.NoError(t, err)
		assert.Equal(t, expectedTask, task)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Failure: task not found in database", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		mockRepo.On("GetByID", mock.Anything, testTaskID, testUserID).Return(nil, repository.ErrTaskNotFound)

		taskSvc := service.NewTaskService(mockRepo, nil)
		task, err := taskSvc.GetTask(ctx, testTaskID, testUserID)

		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
		assert.Nil(t, task)
		mockRepo.AssertExpectations(t)
	})
}

// TestListTasks verifies pagination offset calculations and status validation.
func TestListTasks(t *testing.T) {
	ctx := context.Background()
	testUserID := uuid.New()

	statusTodo := model.StatusTodo
	invalidStatus := model.TaskStatus("archived")

	testCases := []struct {
		name          string
		status        *model.TaskStatus
		page          int
		pageSize      int
		setupMock     func(m *MockTaskRepository)
		expectedErr   error
		expectedCount int
	}{
		{
			name:     "Success: default pagination (page 0 and pageSize 100 clamped)",
			status:   nil,
			page:     0,  // should be clamped to 1
			pageSize: 99, // should be clamped to default 10
			setupMock: func(m *MockTaskRepository) {
				m.On("ListByUserID", mock.Anything, testUserID, (*model.TaskStatus)(nil), 10, 0).
					Return([]model.Task{
						{ID: uuid.New(), UserID: testUserID, Title: "Task 1"},
						{ID: uuid.New(), UserID: testUserID, Title: "Task 2"},
					}, nil)
			},
			expectedErr:   nil,
			expectedCount: 2,
		},
		{
			name:     "Success: page 2 with status filter",
			status:   &statusTodo,
			page:     2,
			pageSize: 5,
			setupMock: func(m *MockTaskRepository) {
				// offset = (2 - 1) * 5 = 5
				m.On("ListByUserID", mock.Anything, testUserID, &statusTodo, 5, 5).
					Return([]model.Task{
						{ID: uuid.New(), UserID: testUserID, Title: "Task 6", Status: model.StatusTodo},
					}, nil)
			},
			expectedErr:   nil,
			expectedCount: 1,
		},
		{
			name:        "Failure: invalid status filter",
			status:      &invalidStatus,
			page:        1,
			pageSize:    10,
			setupMock:   func(m *MockTaskRepository) {},
			expectedErr: service.ErrInvalidTaskStatus,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockTaskRepository)
			tc.setupMock(mockRepo)

			taskSvc := service.NewTaskService(mockRepo, nil)
			tasks, err := taskSvc.ListTasks(ctx, testUserID, tc.status, tc.page, tc.pageSize)

			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, tasks, tc.expectedCount)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestUpdateTaskStatus verifies status transition validations and error propagation.
func TestUpdateTaskStatus(t *testing.T) {
	ctx := context.Background()
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: valid status transition to in_progress", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		mockRepo.On("UpdateStatus", mock.Anything, testTaskID, testUserID, model.StatusInProgress).Return(nil)

		taskSvc := service.NewTaskService(mockRepo, nil)
		err := taskSvc.UpdateTaskStatus(ctx, testTaskID, testUserID, model.StatusInProgress)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Failure: invalid status string", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		taskSvc := service.NewTaskService(mockRepo, nil)

		err := taskSvc.UpdateTaskStatus(ctx, testTaskID, testUserID, "completed")
		assert.ErrorIs(t, err, service.ErrInvalidTaskStatus)
	})

	t.Run("Failure: task not found in database", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		mockRepo.On("UpdateStatus", mock.Anything, testTaskID, testUserID, model.StatusDone).Return(repository.ErrTaskNotFound)

		taskSvc := service.NewTaskService(mockRepo, nil)
		err := taskSvc.UpdateTaskStatus(ctx, testTaskID, testUserID, model.StatusDone)

		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
		mockRepo.AssertExpectations(t)
	})
}

// TestDeleteTask verifies task deletion and error handling.
func TestDeleteTask(t *testing.T) {
	ctx := context.Background()
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: task deleted cleanly", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		mockRepo.On("Delete", mock.Anything, testTaskID, testUserID).Return(nil)

		taskSvc := service.NewTaskService(mockRepo, nil)
		err := taskSvc.DeleteTask(ctx, testTaskID, testUserID)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Failure: task not found or not owned by user", func(t *testing.T) {
		mockRepo := new(MockTaskRepository)
		mockRepo.On("Delete", mock.Anything, testTaskID, testUserID).Return(repository.ErrTaskNotFound)

		taskSvc := service.NewTaskService(mockRepo, nil)
		err := taskSvc.DeleteTask(ctx, testTaskID, testUserID)

		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
		mockRepo.AssertExpectations(t)
	})
}
