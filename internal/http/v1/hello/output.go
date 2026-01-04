package hello

// GetOutput is the response wrapper for the GET hello endpoint.
type GetOutput struct {
	Body Data
}

// CreateOutput is the response wrapper for the POST hello endpoint.
type CreateOutput struct {
	Body Data
}
