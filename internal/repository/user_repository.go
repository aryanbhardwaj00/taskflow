package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/blockaryan/taskflow/internal/infra"
	"github.com/blockaryan/taskflow/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Standard repository errors (sentinel errors)
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("user with this email already exists")
)

// PostgreSQL error code for unique constraint violation
const pgUniqueViolationCode = "23505"

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type PostgresUserRepository struct {
	db *infra.DB
}

func NewPostgresUserRepository(db *infra.DB) *PostgresUserRepository {
	return &PostgresUserRepository{
		db: db,
	}
}

// Create inserts a new user record and scans generated values back into the struct.
func (r *PostgresUserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
            INSERT INTO users (email, password_hash, role)
            VALUES ($1, $2, $3)
            RETURNING id, created_at, updated_at;
        `

	err := r.db.Pool.QueryRow(
		ctx,
		query,
		user.Email,
		user.PasswordHash,
		user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByEmail retrieves a user by their unique email address.
func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
            SELECT id, email, password_hash, role, created_at, updated_at
            FROM users
            WHERE email = $1;
        `

	var user model.User
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by their UUID primary key.
func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
            SELECT id, email, password_hash, role, created_at, updated_at
            FROM users
            WHERE id = $1;
        `

	var user model.User
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}
