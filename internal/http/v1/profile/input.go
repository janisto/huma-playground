package profile

// ProfileCreateInput for POST /profile
type ProfileCreateInput struct {
	Body struct {
		Firstname   string `json:"firstname"    minLength:"1" maxLength:"100" required:"true" doc:"First name"       example:"John"`
		Lastname    string `json:"lastname"     minLength:"1" maxLength:"100" required:"true" doc:"Last name"        example:"Doe"`
		Email       string `json:"email"        format:"email"                required:"true" doc:"Email address"    example:"john@example.com"`
		PhoneNumber string `json:"phoneNumber" pattern:"^\\+[1-9]\\d{6,14}$" required:"true" doc:"Phone (E.164)"    example:"+358401234567"`
		Marketing   bool   `json:"marketing"                                                  doc:"Marketing opt-in" example:"true"`
		Terms       bool   `json:"terms"                                      required:"true" doc:"Terms acceptance" example:"true"`
	}
}

// ProfileGetInput for GET /profile (no body needed)
type ProfileGetInput struct{}

// ProfileUpdateInput for PATCH /profile
type ProfileUpdateInput struct {
	Body struct {
		Firstname   *string `json:"firstname,omitempty"    minLength:"1" maxLength:"100"      doc:"First name"       example:"John"`
		Lastname    *string `json:"lastname,omitempty"     minLength:"1" maxLength:"100"      doc:"Last name"        example:"Doe"`
		Email       *string `json:"email,omitempty"        format:"email"                     doc:"Email address"    example:"john@example.com"`
		PhoneNumber *string `json:"phoneNumber,omitempty" pattern:"^\\+[1-9]\\d{6,14}$"      doc:"Phone (E.164)"    example:"+358401234567"`
		Marketing   *bool   `json:"marketing,omitempty"                                       doc:"Marketing opt-in" example:"true"`
	}
}

// ProfileDeleteInput for DELETE /profile (no body needed)
type ProfileDeleteInput struct{}
