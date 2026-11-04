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
	"github.com/blockaryan/taskflow/internal/middleware"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	// "github.com/blockaryan/taskflow/internal/service"
)

// MockTaskService implements service.TaskService for handler testing.
type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) CreateTask(ctx context.Context, userID uuid.UUID, title, description string, priority model.TaskPriority) (*model.Task, error) {
	args := m.Called(ctx, userID, title, description, priority)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockTaskService) GetTask(ctx context.Context, taskID, userID uuid.UUID) (*model.Task, error) {
	args := m.Called(ctx, taskID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockTaskService) ListTasks(ctx context.Context, userID uuid.UUID, status *model.TaskStatus, page, pageSize int) ([]model.Task, error) {
	args := m.Called(ctx, userID, status, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Task), args.Error(1)
}

func (m *MockTaskService) UpdateTaskStatus(ctx context.Context, taskID, userID uuid.UUID, status model.TaskStatus) error {
	args := m.Called(ctx, taskID, userID, status)
	return args.Error(0)
}

func (m *MockTaskService) DeleteTask(ctx context.Context, taskID, userID uuid.UUID) error {
	args := m.Called(ctx, taskID, userID)
	return args.Error(0)
}

func setupTaskRouter(mockSvc *MockTaskService, currentUserID *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Inject auth context if user is authenticated
	r.Use(func(c *gin.Context) {
		if currentUserID != nil {
			c.Set(middleware.ContextUserIDKey, *currentUserID)
		}
		c.Next()
	})

	h := handler.NewTaskHandler(mockSvc)
	tasks := r.Group("/api/v1/tasks")
	{
		tasks.POST("", h.Create)
		tasks.GET("", h.List)
		tasks.GET("/:id", h.Get)
		tasks.PATCH("/:id/status", h.UpdateStatus)
		tasks.DELETE("/:id", h.Delete)
	}

	return r
}

func TestTaskHandler_Create(t *testing.T) {
	testUserID := uuid.New()

	t.Run("Success: 201 Created", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		expectedTask := &model.Task{
			ID:          uuid.New(),
			UserID:      testUserID,
			Title:       "Test Task",
			Description: "Test Description",
			Status:      model.StatusTodo,
			Priority:    model.PriorityMedium,
			CreatedAt:   time.Now(),
		}

		mockSvc.On("CreateTask", mock.Anything, testUserID, "Test Task", "Test Description", model.PriorityMedium).
			Return(expectedTask, nil)

		payload := handler.CreateTaskRequest{
			Title:       "Test Task",
			Description: "Test Description",
			Priority:    model.PriorityMedium,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Failure: 401 Unauthorized when missing user in context", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, nil) // no auth

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(`{"title":"Task"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Failure: 400 Bad Request on missing title", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(`{"description":"No title"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTaskHandler_Get(t *testing.T) {
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: 200 OK", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		expectedTask := &model.Task{
			ID:     testTaskID,
			UserID: testUserID,
			Title:  "Existing Task",
		}

		mockSvc.On("GetTask", mock.Anything, testTaskID, testUserID).Return(expectedTask, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+testTaskID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Failure: 400 Bad Request on invalid UUID", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/not-a-valid-uuid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Failure: 404 Not Found", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("GetTask", mock.Anything, testTaskID, testUserID).Return(nil, repository.ErrTaskNotFound)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+testTaskID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestTaskHandler_List(t *testing.T) {
	testUserID := uuid.New()

	t.Run("Success: 200 OK", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("ListTasks", mock.Anything, testUserID, (*model.TaskStatus)(nil), 1, 10).
			Return([]model.Task{
				{ID: uuid.New(), Title: "Task 1"},
			}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestTaskHandler_UpdateStatus(t *testing.T) {
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: 200 OK", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("UpdateTaskStatus", mock.Anything, testTaskID, testUserID, model.StatusDone).
			Return(nil)

		payload := handler.UpdateTaskStatusRequest{Status: model.StatusDone}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/"+testTaskID.String()+"/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Failure: 404 Not Found", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("UpdateTaskStatus", mock.Anything, testTaskID, testUserID, model.StatusDone).
			Return(repository.ErrTaskNotFound)

		payload := handler.UpdateTaskStatusRequest{Status: model.StatusDone}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/"+testTaskID.String()+"/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestTaskHandler_Delete(t *testing.T) {
	testUserID := uuid.New()
	testTaskID := uuid.New()

	t.Run("Success: 200 OK", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("DeleteTask", mock.Anything, testTaskID, testUserID).Return(nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+testTaskID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Failure: 404 Not Found", func(t *testing.T) {
		mockSvc := new(MockTaskService)
		router := setupTaskRouter(mockSvc, &testUserID)

		mockSvc.On("DeleteTask", mock.Anything, testTaskID, testUserID).Return(repository.ErrTaskNotFound)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+testTaskID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockSvc.AssertExpectations(t)
	})
}
