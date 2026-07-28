package profile

// ProfileCreateInput for POST /profile
type ProfileCreateInput struct {
	Body struct {
		FirstName    string `json:"firstName"    minLength:"1" maxLength:"100" pattern:"^[^\\s\\p{Z}\\p{C}](?:[^\\p{C}]*[^\\s\\p{Z}\\p{C}])?$" doc:"First name without surrounding whitespace or control characters" example:"John"`
		LastName     string `json:"lastName"     minLength:"1" maxLength:"100" pattern:"^[^\\s\\p{Z}\\p{C}](?:[^\\p{C}]*[^\\s\\p{Z}\\p{C}])?$" doc:"Last name without surrounding whitespace or control characters"  example:"Doe"`
		ContactEmail string `json:"contactEmail" format:"email"                                    doc:"Unverified contact email address" example:"john@example.com"`
		PhoneNumber  string `json:"phoneNumber"  pattern:"^\\+[1-9]\\d{6,14}$"                     doc:"Phone number in E.164 format"      example:"+358401234567"`
		Marketing    bool   `json:"marketing"                                                        doc:"Marketing opt-in"                 example:"true"`
	}
}

// ProfileGetInput for GET /profile (no body needed)
type ProfileGetInput struct{}

// ProfileUpdateInput for PATCH /profile
type ProfileUpdateInput struct {
	Body ProfileUpdateBody `minProperties:"1"`
}

// ProfileUpdateBody contains the fields accepted by PATCH /profile.
type ProfileUpdateBody struct {
	FirstName    *string `json:"firstName,omitempty"    minLength:"1" maxLength:"100" pattern:"^[^\\s\\p{Z}\\p{C}](?:[^\\p{C}]*[^\\s\\p{Z}\\p{C}])?$" doc:"First name without surrounding whitespace or control characters" example:"John"`
	LastName     *string `json:"lastName,omitempty"     minLength:"1" maxLength:"100" pattern:"^[^\\s\\p{Z}\\p{C}](?:[^\\p{C}]*[^\\s\\p{Z}\\p{C}])?$" doc:"Last name without surrounding whitespace or control characters"  example:"Doe"`
	ContactEmail *string `json:"contactEmail,omitempty"                                                                                               doc:"Unverified contact email address"                                example:"john@example.com" format:"email"`
	PhoneNumber  *string `json:"phoneNumber,omitempty"                                pattern:"^\\+[1-9]\\d{6,14}$"                                   doc:"Phone number in E.164 format"                                    example:"+358401234567"`
	Marketing    *bool   `json:"marketing,omitempty"                                                                                                  doc:"Marketing opt-in"                                                example:"true"`
}

// ProfileDeleteInput for DELETE /profile (no body needed)
type ProfileDeleteInput struct{}
