package profile

import (
	"github.com/janisto/huma-playground/internal/platform/timeutil"
)

// Profile represents a user profile response.
type Profile struct {
	ID          string        `json:"id"          doc:"Unique identifier"     example:"user-123"`
	Firstname   string        `json:"firstname"   doc:"First name"            example:"John"`
	Lastname    string        `json:"lastname"    doc:"Last name"             example:"Doe"`
	Email       string        `json:"email"       doc:"Email address"         example:"john@example.com"`
	PhoneNumber string        `json:"phoneNumber" doc:"Phone number (E.164)"  example:"+358401234567"`
	Marketing   bool          `json:"marketing"   doc:"Marketing opt-in"      example:"true"`
	Terms       bool          `json:"terms"       doc:"Terms acceptance"      example:"true"`
	CreatedAt   timeutil.Time `json:"createdAt"   doc:"Creation timestamp"    example:"2024-01-15T10:30:00.000Z"`
	UpdatedAt   timeutil.Time `json:"updatedAt"   doc:"Last update timestamp" example:"2024-01-15T10:30:00.000Z"`
}
