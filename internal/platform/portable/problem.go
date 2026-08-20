package portable

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
)

// Code is a stable portable error code.
type Code string

const (
	CodeInvalidRequest        Code = "invalid_request"
	CodeUnauthorized          Code = "unauthorized"
	CodeForbidden             Code = "forbidden"
	CodeNotFound              Code = "not_found"
	CodeProfileNotFound       Code = "profile_not_found"
	CodeGitHubNotFound        Code = "github_not_found"
	CodeMethodNotAllowed      Code = "method_not_allowed"
	CodeNotAcceptable         Code = "not_acceptable"
	CodeProfileExists         Code = "profile_exists"
	CodePayloadTooLarge       Code = "payload_too_large"
	CodeUnsupportedMediaType  Code = "unsupported_media_type"
	CodeValidationFailed      Code = "validation_failed"
	CodeRateLimited           Code = "rate_limited"
	CodeGitHubRateLimit       Code = "github_rate_limit"
	CodeInternalError         Code = "internal_error"
	CodeGitHubUpstream        Code = "github_upstream"
	CodeDependencyUnavailable Code = "dependency_unavailable"
	CodeGitHubTimeout         Code = "github_timeout"
)

type problemDefinition struct {
	Status int
	Title  string
	Detail string
}

var problemDefinitions = map[Code]problemDefinition{
	CodeInvalidRequest:   {http.StatusBadRequest, "Bad Request", "Request is malformed"},
	CodeUnauthorized:     {http.StatusUnauthorized, "Unauthorized", "Authentication is required or invalid"},
	CodeForbidden:        {http.StatusForbidden, "Forbidden", "Access is forbidden"},
	CodeNotFound:         {http.StatusNotFound, "Not Found", "Resource not found"},
	CodeProfileNotFound:  {http.StatusNotFound, "Not Found", "Profile not found"},
	CodeGitHubNotFound:   {http.StatusNotFound, "Not Found", "GitHub resource not found"},
	CodeMethodNotAllowed: {http.StatusMethodNotAllowed, "Method Not Allowed", "Method not allowed"},
	CodeNotAcceptable: {
		http.StatusNotAcceptable,
		"Not Acceptable",
		"No acceptable response representation is available",
	},
	CodeProfileExists:   {http.StatusConflict, "Conflict", "Profile already exists"},
	CodePayloadTooLarge: {http.StatusRequestEntityTooLarge, "Content Too Large", "Request body is too large"},
	CodeUnsupportedMediaType: {
		http.StatusUnsupportedMediaType,
		"Unsupported Media Type",
		"Request representation is not supported",
	},
	CodeValidationFailed: {http.StatusUnprocessableEntity, "Unprocessable Content", "Request validation failed"},
	CodeRateLimited:      {http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"},
	CodeGitHubRateLimit:  {http.StatusTooManyRequests, "Too Many Requests", "GitHub rate limit exceeded"},
	CodeInternalError:    {http.StatusInternalServerError, "Internal Server Error", "Internal server error"},
	CodeGitHubUpstream: {
		http.StatusBadGateway,
		"Bad Gateway",
		"GitHub upstream response is invalid or unavailable",
	},
	CodeDependencyUnavailable: {
		http.StatusServiceUnavailable,
		"Service Unavailable",
		"A required dependency is unavailable",
	},
	CodeGitHubTimeout: {http.StatusGatewayTimeout, "Gateway Timeout", "GitHub request timed out"},
}

// Source identifies a known structural input location without reflecting a rejected value.
type Source struct {
	Pointer   *string `json:"pointer,omitempty"   nullable:"false" maxLength:"256"`
	Parameter *string `json:"parameter,omitempty" nullable:"false" maxLength:"256"`
	Header    *string `json:"header,omitempty"    nullable:"false" maxLength:"256"`
}

// Issue is a normalized validation issue.
type Issue struct {
	Detail string  `json:"detail"           minLength:"1" maxLength:"200"`
	Source *Source `json:"source,omitempty"                               nullable:"false"`
}

// ProblemError is the closed GCP RFC 9457 response model and Huma status error.
type ProblemError struct {
	Title     string  `json:"title"`
	Status    int     `json:"status"           minimum:"400" maximum:"599"`
	Detail    string  `json:"detail"`
	Code      Code    `json:"code"                                         pattern:"^[a-z][a-z0-9_]*$" enum:"invalid_request,unauthorized,forbidden,not_found,profile_not_found,github_not_found,method_not_allowed,not_acceptable,profile_exists,payload_too_large,unsupported_media_type,validation_failed,rate_limited,github_rate_limit,internal_error,github_upstream,dependency_unavailable,github_timeout"`
	Errors    []Issue `json:"errors,omitempty"                                                                                                                                                                                                                                                                                                                                                                    nullable:"false" maxItems:"32"`
	mediaType string
}

func (p *ProblemError) Error() string { return p.Detail }

func (p *ProblemError) GetStatus() int { return p.Status }

// ContentType keeps RFC 9457 JSON while using ordinary application/cbor for CBOR.
func (p *ProblemError) ContentType(contentType string) string {
	if p.mediaType != "" {
		return p.mediaType
	}
	lower := strings.ToLower(contentType)
	if strings.HasPrefix(lower, "application/cbor") {
		return "application/cbor"
	}
	if strings.Contains(lower, "charset=utf-8") {
		return "application/problem+json; charset=utf-8"
	}
	return "application/problem+json"
}

type originalAcceptKey struct{}

// WithOriginalAccept preserves caller negotiation before policy rewrites the
// header to the already-selected success representation.
func WithOriginalAccept(ctx context.Context, accept string) context.Context {
	return context.WithValue(ctx, originalAcceptKey{}, accept)
}

func originalAccept(ctx context.Context) string {
	accept, _ := ctx.Value(originalAcceptKey{}).(string)
	return accept
}

// NewProblem returns the exact public status, title, and detail for code.
func NewProblem(code Code, issues ...Issue) *ProblemError {
	definition, ok := problemDefinitions[code]
	if !ok {
		code = CodeInternalError
		definition = problemDefinitions[code]
	}
	return &ProblemError{
		Title:  definition.Title,
		Status: definition.Status,
		Detail: definition.Detail,
		Code:   code,
		Errors: boundedIssues(issues),
	}
}

// NewProblemForContext selects an error representation from the caller's
// original Accept field, independently of success negotiation.
func NewProblemForContext(ctx context.Context, code Code, issues ...Issue) *ProblemError {
	problem := NewProblem(code, issues...)
	problem.mediaType = NegotiateProblem(originalAccept(ctx))
	return problem
}

func boundedIssues(issues []Issue) []Issue {
	if len(issues) <= 32 {
		return issues
	}
	bounded := append([]Issue(nil), issues[:31]...)
	return append(bounded, Issue{Detail: "Additional validation errors omitted"})
}

// Error returns a Huma-compatible portable status error.
func Error(code Code, issues ...Issue) huma.StatusError {
	return NewProblem(code, issues...)
}

func ErrorForContext(ctx context.Context, code Code, issues ...Issue) huma.StatusError {
	return NewProblemForContext(ctx, code, issues...)
}

var configureHumaOnce sync.Once

// ConfigureHuma installs the application-wide portable error model used by
// Huma's parser, validator, generated responses, and helper functions.
func ConfigureHuma() {
	configureHumaOnce.Do(func() {
		factory := func(status int, _ string, errs ...error) huma.StatusError {
			code := genericCodeForStatus(status)
			if code == CodeValidationFailed {
				return NewProblem(code, normalizeValidationIssues(errs)...)
			}
			return NewProblem(code)
		}
		huma.NewError = factory
		huma.NewErrorWithContext = func(ctx huma.Context, status int, _ string, errs ...error) huma.StatusError {
			code := genericCodeForStatus(status)
			if code == CodeValidationFailed {
				return NewProblemForContext(ctx.Context(), code, normalizeValidationIssues(errs)...)
			}
			return NewProblemForContext(ctx.Context(), code)
		}
	})
}

func genericCodeForStatus(status int) Code {
	switch status {
	case http.StatusBadRequest:
		return CodeInvalidRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusMethodNotAllowed:
		return CodeMethodNotAllowed
	case http.StatusNotAcceptable:
		return CodeNotAcceptable
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusUnsupportedMediaType:
		return CodeUnsupportedMediaType
	case http.StatusUnprocessableEntity:
		return CodeValidationFailed
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusBadGateway:
		return CodeGitHubUpstream
	case http.StatusServiceUnavailable:
		return CodeDependencyUnavailable
	case http.StatusGatewayTimeout:
		return CodeGitHubTimeout
	default:
		return CodeInternalError
	}
}

var knownSourceNames = map[string]struct{}{
	"name": {}, "firstName": {}, "lastName": {}, "contactEmail": {},
	"phoneNumber": {}, "marketingOptIn": {}, "termsAccepted": {},
	"owner": {}, "repo": {}, "limit": {}, "cursor": {}, "category": {},
	"Content-Type": {}, "Content-Encoding": {}, "Authorization": {}, "X-Request-ID": {},
}

func normalizeValidationIssues(errs []error) []Issue {
	issues := make([]Issue, 0, min(len(errs), 32))
	for _, err := range errs {
		var detail *huma.ErrorDetail
		if !errors.As(err, &detail) || detail == nil {
			issues = append(issues, Issue{Detail: "Invalid request value"})
			continue
		}
		issues = append(issues, normalizeValidationIssue(detail.Location))
	}
	return boundedIssues(issues)
}

func normalizeValidationIssue(location string) Issue {
	parts := strings.FieldsFunc(location, func(r rune) bool {
		return r == '.' || r == '[' || r == ']'
	})
	if len(parts) < 2 {
		return Issue{Detail: "Invalid request body"}
	}
	name := parts[1]
	if _, ok := knownSourceNames[name]; !ok {
		return Issue{Detail: "Invalid request value"}
	}
	switch parts[0] {
	case "body":
		pointer := "/" + name
		return Issue{Detail: "Invalid request body member", Source: &Source{Pointer: &pointer}}
	case "query":
		return Issue{Detail: "Invalid query parameter", Source: &Source{Parameter: &name}}
	case "path":
		return Issue{Detail: "Invalid path parameter", Source: &Source{Parameter: &name}}
	case "header":
		canonical := http.CanonicalHeaderKey(name)
		if _, ok := knownSourceNames[canonical]; !ok {
			return Issue{Detail: "Invalid request header"}
		}
		return Issue{Detail: "Invalid request header", Source: &Source{Header: &canonical}}
	default:
		return Issue{Detail: "Invalid request value"}
	}
}
