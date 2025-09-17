package api

// Envelope is a generic API response envelope wrapping all responses for consistency.
// data: the primary response data (null for errors or empty responses)
// meta: trace correlation and any pagination, etc.
// error: populated only on failures.
type Envelope[T any] struct {
	Data  *T         `json:"data"`
	Meta  Meta       `json:"meta"`
	Error *ErrorBody `json:"error"`
}

// Meta holds cross-cutting metadata.
type Meta struct {
	TraceID *string `json:"traceId,omitempty"`
}

// ErrorBody describes an error in a predictable structured format.
type ErrorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldIssue `json:"details,omitempty"`
	TraceID *string      `json:"traceId,omitempty"`
}

// FieldIssue gives field-level or contextual error information.
type FieldIssue struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue"`
}

// NewSuccessEnvelope constructs a success envelope.
func NewSuccessEnvelope[T any](traceID *string, data T) Envelope[T] {
	d := data
	return Envelope[T]{
		Data: &d,
		Meta: Meta{TraceID: traceID},
	}
}

// NewErrorEnvelope constructs an error envelope with no data.
func NewErrorEnvelope[T any](traceID *string, code, msg string, details []FieldIssue) Envelope[T] {
	var clonedDetails []FieldIssue
	if len(details) > 0 {
		clonedDetails = make([]FieldIssue, len(details))
		copy(clonedDetails, details)
	}
	return Envelope[T]{
		Data: nil,
		Meta: Meta{TraceID: traceID},
		Error: &ErrorBody{
			Code:    code,
			Message: msg,
			Details: clonedDetails,
			TraceID: traceID,
		},
	}
}
