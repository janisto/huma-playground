package profile

import (
	"github.com/janisto/huma-playground/internal/platform/timeutil"
)

// Profile represents a user profile response.
type Profile struct {
	ID             string        `json:"id"             minLength:"1" maxLength:"128" doc:"Verified opaque principal identifier" example:"principal-123"`
	FirstName      string        `json:"firstName"      minLength:"1" maxLength:"100" doc:"First name"                           example:"Ada"`
	LastName       string        `json:"lastName"       minLength:"1" maxLength:"100" doc:"Last name"                            example:"Lovelace"`
	ContactEmail   string        `json:"contactEmail"   minLength:"3" maxLength:"254" doc:"Canonical ASCII contact address"      example:"ada@example.com"`
	PhoneNumber    string        `json:"phoneNumber"                                  doc:"Canonical E.164 phone number"         example:"+358401234567"            pattern:"^\\+[1-9][0-9]{6,14}$"`
	MarketingOptIn bool          `json:"marketingOptIn"                               doc:"Marketing opt-in"                     example:"false"`
	TermsAccepted  bool          `json:"termsAccepted"                                doc:"Accepted terms"                       example:"true"                                                     enum:"true"`
	CreatedAt      timeutil.Time `json:"createdAt"                                    doc:"Creation timestamp"                   example:"2026-07-30T12:00:00.000Z"                                             format:"date-time"`
	UpdatedAt      timeutil.Time `json:"updatedAt"                                    doc:"Last update timestamp"                example:"2026-07-30T12:05:00.000Z"                                             format:"date-time"`
}
