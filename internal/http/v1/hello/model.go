package hello

// Data models the response payload for hello endpoints.
type Data struct {
	Message string `json:"message" doc:"Greeting message" example:"Hello, World!"`
}
