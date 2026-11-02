package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// TaskRepository defines database operations for tasks.
type TaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByID(ctx context.Context, taskID, userID uuid.UUID) (*model.Task, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, status *model.TaskStatus, limit, offset int) ([]model.Task, error)
	UpdateStatus(ctx context.Context, taskID, userID uuid.UUID, status model.TaskStatus) error
	Delete(ctx context.Context, taskID, userID uuid.UUID) error
}

// PostgresTaskRepository implements TaskRepository using PostgreSQL.
type PostgresTaskRepository struct {
	db *infra.DB
}

// NewPostgresTaskRepository constructs a new PostgresTaskRepository.
func NewPostgresTaskRepository(db *infra.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{db: db}
}

// Create inserts a new task record.
func (r *PostgresTaskRepository) Create(ctx context.Context, task *model.Task) error {
	query := `
            INSERT INTO tasks (user_id, title, description, status, priority)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING id, created_at, updated_at;
        `

	err := r.db.Pool.QueryRow(
		ctx,
		query,
		task.UserID,
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
	).Scan(&task.ID, &task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetByID retrieves a task ensuring it belongs to the authenticated user (IDOR prevention).
func (r *PostgresTaskRepository) GetByID(ctx context.Context, taskID, userID uuid.UUID) (*model.Task, error) {
	query := `
            SELECT id, user_id, title, description, status, priority, created_at, updated_at
            FROM tasks
            WHERE id = $1 AND user_id = $2;
        `

	var task model.Task
	err := r.db.Pool.QueryRow(ctx, query, taskID, userID).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return &task, nil
}

// ListByUserID retrieves paginated tasks belonging to a user, optionally filtered by status.
func (r *PostgresTaskRepository) ListByUserID(
	ctx context.Context,
	userID uuid.UUID,
	status *model.TaskStatus,
	limit, offset int,
) ([]model.Task, error) {
	var query string
	var args []any

	if status != nil {
		query = `
                SELECT id, user_id, title, description, status, priority, created_at, updated_at
                FROM tasks
                WHERE user_id = $1 AND status = $2
                ORDER BY created_at DESC
                LIMIT $3 OFFSET $4;
            `
		args = []any{userID, *status, limit, offset}
	} else {
		query = `
                SELECT id, user_id, title, description, status, priority, created_at, updated_at
                FROM tasks
                WHERE user_id = $1
                ORDER BY created_at DESC
                LIMIT $2 OFFSET $3;
            `
		args = []any{userID, limit, offset}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		var task model.Task
		if err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Priority,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tasks, nil
}

// UpdateStatus changes the status of a task belonging to the user.
func (r *PostgresTaskRepository) UpdateStatus(
	ctx context.Context,
	taskID, userID uuid.UUID,
	status model.TaskStatus,
) error {
	query := `
            UPDATE tasks
            SET status = $1, updated_at = CURRENT_TIMESTAMP
            WHERE id = $2 AND user_id = $3;
        `

	tag, err := r.db.Pool.Exec(ctx, query, status, taskID, userID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// Delete removes a task belonging to the user.
func (r *PostgresTaskRepository) Delete(ctx context.Context, taskID, userID uuid.UUID) error {
	query := `
            DELETE FROM tasks
            WHERE id = $1 AND user_id = $2;
        `

	tag, err := r.db.Pool.Exec(ctx, query, taskID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}

	return nil
}
