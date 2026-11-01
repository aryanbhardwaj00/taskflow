package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// In Phase 3, we will establish and configure the PostgreSQL connection pool using pgx.

// DB wraps pgxpool.Pool to provide database connection management.
type DB struct {
	Pool *pgxpool.Pool
}

func NewPostgresDB(cfg config.DBConfig) (*DB, error) {
	// Construct PostgreSQL connection string (DSN)
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.SSLMode,
	)

	// Parse connection string into a pgxpool configuration struct
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Connection Pool Settings
	poolConfig.MaxConns = 25                      // Maximum number of connections in the pool
	poolConfig.MinConns = 5                       // Minimum number of idle connections maintained
	poolConfig.MaxConnLifetime = 1 * time.Hour    // Maximum time a connection may be reused
	poolConfig.MaxConnIdleTime = 15 * time.Minute // Maximum time a connection may remain idle

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Initialize connection pool

	// Initialize the connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Verify database connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Printf("Connected to PostgreSQL at %s:%s/%s successfully", cfg.Host, cfg.Port, cfg.Name)

	return &DB{Pool: pool}, nil
}

// Close gracefully closes all active database connections in the pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Println("PostgreSQL connection pool closed.")
	}
}
