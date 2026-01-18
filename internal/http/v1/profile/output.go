package profile

// ProfileCreateOutput for POST /profile (201 Created)
type ProfileCreateOutput struct {
	Location string `header:"Location" doc:"URL of created profile"`
	Body     Profile
}

// ProfileGetOutput for GET /profile
type ProfileGetOutput struct {
	Body Profile
}

// ProfileUpdateOutput for PATCH /profile
type ProfileUpdateOutput struct {
	Body Profile
}
