package hello

// HelloGetOutput is the response wrapper for the GET hello endpoint.
type HelloGetOutput struct {
	Body Data
}

// HelloCreateOutput is the response wrapper for the POST hello endpoint.
type HelloCreateOutput struct {
	Body Data
}
