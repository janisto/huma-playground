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
)

// Profile represents stored profile data.
type Profile struct {
	ID          string
	Firstname   string
	Lastname    string
	Email       string
	PhoneNumber string
	Marketing   bool
	Terms       bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateParams for creating a profile.
type CreateParams struct {
	Firstname   string
	Lastname    string
	Email       string
	PhoneNumber string
	Marketing   bool
	Terms       bool
}

// UpdateParams for updating a profile.
type UpdateParams struct {
	Firstname   *string
	Lastname    *string
	Email       *string
	PhoneNumber *string
	Marketing   *bool
}

// Service defines profile operations.
//
// Implementations must normalize input data:
//   - Email: lowercase and trim whitespace
//   - PhoneNumber: trim whitespace
type Service interface {
	Create(ctx context.Context, userID string, params CreateParams) (*Profile, error)
	Get(ctx context.Context, userID string) (*Profile, error)
	Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error)
	Delete(ctx context.Context, userID string) error
}
