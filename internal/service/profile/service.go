package profile

import (
	"context"
	"errors"
	"time"
)

// Service errors
var (
	ErrNotFound      = errors.New("profile not found")
	ErrAlreadyExists = errors.New("profile already exists")
	ErrUnavailable   = errors.New("profile store unavailable")
)

// Profile represents stored profile data.
type Profile struct {
	ID           string
	FirstName    string
	LastName     string
	ContactEmail string
	PhoneNumber  string
	Marketing    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateParams for creating a profile.
type CreateParams struct {
	FirstName    string
	LastName     string
	ContactEmail string
	PhoneNumber  string
	Marketing    bool
}

// UpdateParams for updating a profile.
type UpdateParams struct {
	FirstName    *string
	LastName     *string
	ContactEmail *string
	PhoneNumber  *string
	Marketing    *bool
}

// Store defines profile persistence operations.
type Store interface {
	Create(ctx context.Context, userID string, params CreateParams) (*Profile, error)
	Get(ctx context.Context, userID string) (*Profile, error)
	Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error)
	Delete(ctx context.Context, userID string) error
}
