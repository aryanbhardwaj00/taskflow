package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents the domain model for a user account.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // "-" ensures password hash is NEVER leaked in JSON responses
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
