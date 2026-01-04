package hello

// CreateInput is the request body for creating a greeting.
type CreateInput struct {
	Body struct {
		Name string `json:"name" doc:"Name to greet" example:"World" minLength:"1" maxLength:"100"`
	}
}
