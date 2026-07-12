package profile

import (
	"github.com/janisto/huma-playground/internal/platform/timeutil"
)

// Profile represents a user profile response.
type Profile struct {
	ID           string        `json:"id"           doc:"Unique identifier"                example:"user-123"`
	FirstName    string        `json:"firstName"    doc:"First name"                       example:"John"`
	LastName     string        `json:"lastName"     doc:"Last name"                        example:"Doe"`
	ContactEmail string        `json:"contactEmail" doc:"Unverified contact email address" example:"john@example.com"`
	PhoneNumber  string        `json:"phoneNumber"  doc:"Phone number in E.164 format"     example:"+358401234567"`
	Marketing    bool          `json:"marketing"    doc:"Marketing opt-in"                 example:"true"`
	CreatedAt    timeutil.Time `json:"createdAt"    doc:"Creation timestamp"               example:"2024-01-15T10:30:00.000Z"`
	UpdatedAt    timeutil.Time `json:"updatedAt"    doc:"Last update timestamp"            example:"2024-01-15T10:30:00.000Z"`
}
