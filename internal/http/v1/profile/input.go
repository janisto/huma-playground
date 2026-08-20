package profile

// ProfileCreateInput for POST /profile
type ProfileCreateInput struct {
	Body struct {
		FirstName      string `json:"firstName" minLength:"1" maxLength:"100" doc:"First name without surrounding portable whitespace or controls" example:"Ada"`
		LastName       string `json:"lastName" minLength:"1" maxLength:"100" doc:"Last name without surrounding portable whitespace or controls" example:"Lovelace"`
		ContactEmail   string `json:"contactEmail" doc:"ASCII contact address; surrounding ASCII whitespace is removed and the domain is lowercased" example:"ada@example.com"`
		PhoneNumber    string `json:"phoneNumber" doc:"E.164 phone number; surrounding ASCII whitespace is removed" example:"+358401234567"`
		MarketingOptIn *bool  `json:"marketingOptIn,omitempty" nullable:"false" default:"false" doc:"Marketing opt-in" example:"false"`
		TermsAccepted  bool   `json:"termsAccepted" enum:"true" doc:"Required terms acceptance" example:"true"`
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
	FirstName      *string `json:"firstName,omitempty"      nullable:"false" minLength:"1" maxLength:"100" doc:"First name without surrounding portable whitespace or controls"                              example:"Ada"`
	LastName       *string `json:"lastName,omitempty"       nullable:"false" minLength:"1" maxLength:"100" doc:"Last name without surrounding portable whitespace or controls"                               example:"Lovelace"`
	ContactEmail   *string `json:"contactEmail,omitempty"   nullable:"false"                               doc:"ASCII contact address; surrounding ASCII whitespace is removed and the domain is lowercased" example:"ada@example.com"`
	PhoneNumber    *string `json:"phoneNumber,omitempty"    nullable:"false"                               doc:"E.164 phone number; surrounding ASCII whitespace is removed"                                 example:"+358401234567"`
	MarketingOptIn *bool   `json:"marketingOptIn,omitempty" nullable:"false"                               doc:"Marketing opt-in"                                                                            example:"true"`
}

// ProfileDeleteInput for DELETE /profile (no body needed)
type ProfileDeleteInput struct{}
