package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/blockaryan/taskflow/internal/middleware"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
	"github.com/blockaryan/taskflow/internal/service"
)

// CreateTaskRequest defines the input payload for creating a task.
type CreateTaskRequest struct {
	Title       string             `json:"title" binding:"required"`
	Description string             `json:"description"`
	Priority    model.TaskPriority `json:"priority"`
}

// UpdateTaskStatusRequest defines the input payload for moving task status.
type UpdateTaskStatusRequest struct {
	Status model.TaskStatus `json:"status" binding:"required"`
}

// TaskHandler handles HTTP requests for task management.
type TaskHandler struct {
	taskService service.TaskService
}

// NewTaskHandler constructs a new TaskHandler with service dependency injection.
func NewTaskHandler(taskService service.TaskService) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
	}
}

// Create handles POST /api/v1/tasks
func (h *TaskHandler) Create(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	task, err := h.taskService.CreateTask(c.Request.Context(), userID, req.Title, req.Description, req.Priority)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaskTitle) || errors.Is(err, service.ErrInvalidTaskPriority) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Task created successfully",
		"task":    task,
	})
}

// List handles GET /api/v1/tasks?status=todo&page=1&limit=10
func (h *TaskHandler) List(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	var statusFilter *model.TaskStatus
	if rawStatus := c.Query("status"); rawStatus != "" {
		s := model.TaskStatus(rawStatus)
		statusFilter = &s
	}

	tasks, err := h.taskService.ListTasks(c.Request.Context(), userID, statusFilter, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaskStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
		"page":  page,
		"limit": limit,
	})
}

// Get handles GET /api/v1/tasks/:id
func (h *TaskHandler) Get(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	task, err := h.taskService.GetTask(c.Request.Context(), taskID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": task})
}

// UpdateStatus handles PATCH /api/v1/tasks/:id/status
func (h *TaskHandler) UpdateStatus(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	var req UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	if err := h.taskService.UpdateTaskStatus(c.Request.Context(), taskID, userID, req.Status); err != nil {
		switch {
		case errors.Is(err, repository.ErrTaskNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		case errors.Is(err, service.ErrInvalidTaskStatus):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task status"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task status updated successfully"})
}

// Delete handles DELETE /api/v1/tasks/:id
func (h *TaskHandler) Delete(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	if err := h.taskService.DeleteTask(c.Request.Context(), taskID, userID); err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task deleted successfully"})
}
