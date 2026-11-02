package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/blockaryan/taskflow/internal/repository"
)

const (
	taskCacheTTL = 10 * time.Minute
)

var (
	ErrInvalidTaskTitle    = errors.New("task title cannot be empty")
	ErrInvalidTaskStatus   = errors.New("invalid task status: must be 'todo', 'in_progress', or 'done'")
	ErrInvalidTaskPriority = errors.New("invalid task priority: must be 'low', 'medium', or 'high'")
)

// TaskService defines business operations for managing tasks.
type TaskService interface {
	CreateTask(ctx context.Context, userID uuid.UUID, title, description string, priority model.TaskPriority) (*model.Task, error)
	GetTask(ctx context.Context, taskID, userID uuid.UUID) (*model.Task, error)
	ListTasks(ctx context.Context, userID uuid.UUID, status *model.TaskStatus, page, pageSize int) ([]model.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID, userID uuid.UUID, status model.TaskStatus) error
	DeleteTask(ctx context.Context, taskID, userID uuid.UUID) error
}

// taskService is the concrete implementation of TaskService.
type taskService struct {
	taskRepo repository.TaskRepository
	redis    *infra.RedisClient
}

// NewTaskService constructs a new TaskService with repository dependency injection.
func NewTaskService(taskRepo repository.TaskRepository, redis *infra.RedisClient) TaskService {
	return &taskService{
		taskRepo: taskRepo,
		redis:    redis,
	}
}

// taskCacheKey generates a consistent Redis key for a task.
func taskCacheKey(taskID uuid.UUID) string {
	return fmt.Sprintf("task:%s", taskID.String())
}

// CreateTask validates input, applies defaults, and persists a new task.
func (s *taskService) CreateTask(ctx context.Context, userID uuid.UUID, title, description string, priority model.TaskPriority) (*model.Task, error) {
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		return nil, ErrInvalidTaskTitle
	}

	// Validate or default priority
	if priority == "" {
		priority = model.PriorityMedium
	} else if priority != model.PriorityLow && priority != model.PriorityMedium && priority != model.PriorityHigh {
		return nil, ErrInvalidTaskPriority
	}

	task := &model.Task{
		UserID:      userID,
		Title:       cleanTitle,
		Description: strings.TrimSpace(description),
		Status:      model.StatusTodo, // default status for new tasks
		Priority:    priority,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// GetTask retrieves a single task ensuring user ownership.
func (s *taskService) GetTask(ctx context.Context, taskID, userID uuid.UUID) (*model.Task, error) {
	cacheKey := taskCacheKey(taskID)
	fmt.Println("Cached Json:", cacheKey)

	// Attemt to read from Redis Cache
	cachedJson, err := s.redis.Get(ctx, cacheKey)
	fmt.Println("Cached Json:", cachedJson)
	if err == nil {
		var task model.Task
		if err := json.Unmarshal([]byte(cachedJson), &task); err == nil {
			// Verify ownership on cached task
			if task.UserID == userID {
				log.Printf("CACHE HIT: retrieved task %s from Redis\n", taskID)
				return &task, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			// Log Redis connection error, but don't fail! Gracefully degrade to PostgreSQL.
			log.Printf("CACHE WARNING: Redis error on Get: %v\n", err)
		}
	}
	// 2. Cache Miss: Query PostgreSQL (Source of Truth)
	log.Printf("CACHE MISS: querying PostgreSQL for task %s\n", taskID)
	task, err := s.taskRepo.GetByID(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}

	// 3. Populate Redis cache asynchronously or inline with TTL
	if data, err := json.Marshal(task); err == nil {
		if err := s.redis.Set(ctx, cacheKey, string(data), taskCacheTTL); err != nil {
			log.Printf("CACHE WARNING: failed to cache task in Redis: %v\n", err)
		}
	}

	return task, nil
}

// ListTasks calculates safe pagination offsets and fetches user tasks.
func (s *taskService) ListTasks(ctx context.Context, userID uuid.UUID, status *model.TaskStatus, page, pageSize int) ([]model.Task, error) {
	// Guard against pagination abuse
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10 // default page size
	}

	offset := (page - 1) * pageSize

	// Validate status filter if provided
	if status != nil {
		if *status != model.StatusTodo && *status != model.StatusInProgress && *status != model.StatusDone {
			return nil, ErrInvalidTaskStatus
		}
	}

	return s.taskRepo.ListByUserID(ctx, userID, status, pageSize, offset)
}

// UpdateTaskStatus updates PostgreSQL and invalidates the Redis cache.
func (s *taskService) UpdateTaskStatus(
	ctx context.Context,
	taskID, userID uuid.UUID,
	status model.TaskStatus,
) error {
	if status != model.StatusTodo && status != model.StatusInProgress && status != model.StatusDone {
		return ErrInvalidTaskStatus
	}

	// 1. Update database
	if err := s.taskRepo.UpdateStatus(ctx, taskID, userID, status); err != nil {
		return err
	}

	// 2. Invalidate cache: Delete key from Redis
	if err := s.redis.Del(ctx, taskCacheKey(taskID)); err != nil {
		log.Printf("CACHE WARNING: failed to invalidate cache for task %s: %v\n", taskID, err)
	} else {
		log.Printf("CACHE INVALIDATED: removed task %s from Redis\n", taskID)
	}

	return nil
}

// DeleteTask removes task from PostgreSQL and invalidates the Redis cache.
func (s *taskService) DeleteTask(ctx context.Context, taskID, userID uuid.UUID) error {
	// 1. Delete from database
	if err := s.taskRepo.Delete(ctx, taskID, userID); err != nil {
		return err
	}

	// 2. Invalidate cache: Delete key from Redis
	if err := s.redis.Del(ctx, taskCacheKey(taskID)); err != nil {
		log.Printf("CACHE WARNING: failed to delete cached task %s: %v\n", taskID, err)
	} else {
		log.Printf("CACHE INVALIDATED: removed task %s from Redis\n", taskID)
	}

	return nil
}
