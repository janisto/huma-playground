package portable

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxRequestBodyBytes int64 = 1_000_000

type queryKind uint8

const (
	queryNone queryKind = iota
	queryPagination
	queryItems
)

type routePolicy struct {
	query        queryKind
	jsonOnly     bool
	body         bool
	bodyless     bool
	successMedia bool
}

// RequestPolicy enforces the portable structural boundary before Huma auth,
// decoding, handlers, persistence, or GitHub calls.
func RequestPolicy(apiPrefix string) func(http.Handler) http.Handler {
	return requestPolicy(apiPrefix, nil)
}

// RequestPolicyWithRejectionMiddleware observes policy-owned responses without
// wrapping successful Huma requests in a second HTTP middleware lifecycle.
func RequestPolicyWithRejectionMiddleware(
	apiPrefix string,
	rejectionMiddleware func(http.Handler) http.Handler,
) func(http.Handler) http.Handler {
	return requestPolicy(apiPrefix, rejectionMiddleware)
}

func requestPolicy(
	apiPrefix string,
	rejectionMiddleware func(http.Handler) http.Handler,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			path := request.URL.Path
			if apiPrefix != "/v1" && strings.HasPrefix(path, apiPrefix+"/") {
				path = "/v1" + strings.TrimPrefix(path, apiPrefix)
			}
			policy, matched := policyFor(request.Method, path)
			if !matched {
				next.ServeHTTP(w, request)
				return
			}

			originalAccept := AcceptHeader(request.Header)
			request = request.Clone(WithOriginalAccept(request.Context(), originalAccept))

			if code := validateRawQuery(request.URL.RawQuery, policy.query); code != "" {
				writePolicyProblem(w, request, code, rejectionMiddleware)
				return
			}
			if policy.body {
				if code := validateDeclaredLength(request); code != "" {
					writePolicyProblem(w, request, code, rejectionMiddleware)
					return
				}
				if code := validateRequestRepresentation(request); code != "" {
					writePolicyProblem(w, request, code, rejectionMiddleware)
					return
				}
			}
			if policy.successMedia && !policy.bodyless {
				selected, ok := NegotiateSuccess(originalAccept, policy.jsonOnly)
				if !ok {
					writePolicyProblem(w, request, CodeNotAcceptable, rejectionMiddleware)
					return
				}
				request.Header["Accept"] = []string{selected}
			}
			next.ServeHTTP(w, request)
		})
	}
}

func writePolicyProblem(
	response http.ResponseWriter,
	request *http.Request,
	code Code,
	rejectionMiddleware func(http.Handler) http.Handler,
) {
	handler := http.Handler(http.HandlerFunc(func(writer http.ResponseWriter, current *http.Request) {
		WriteProblem(writer, current, code)
	}))
	if rejectionMiddleware != nil {
		handler = rejectionMiddleware(handler)
	}
	handler.ServeHTTP(response, request)
}

func policyFor(method, path string) (routePolicy, bool) {
	switch {
	case method == http.MethodGet && path == "/health":
		return routePolicy{query: queryNone, successMedia: true}, true
	case method == http.MethodGet && path == "/openapi.json":
		return routePolicy{query: queryNone, jsonOnly: true, successMedia: true}, true
	case path == "/v1/hello" && method == http.MethodGet:
		return routePolicy{query: queryNone, successMedia: true}, true
	case path == "/v1/hello" && method == http.MethodPost:
		return routePolicy{query: queryNone, body: true, successMedia: true}, true
	case method == http.MethodGet && path == "/v1/items":
		return routePolicy{query: queryItems, successMedia: true}, true
	case path == "/v1/profile" && (method == http.MethodPost || method == http.MethodPatch):
		return routePolicy{query: queryNone, body: true, successMedia: true}, true
	case path == "/v1/profile" && method == http.MethodGet:
		return routePolicy{query: queryNone, successMedia: true}, true
	case path == "/v1/profile" && method == http.MethodDelete:
		return routePolicy{query: queryNone, bodyless: true}, true
	case method == http.MethodGet && isGitHubPointPath(path):
		return routePolicy{query: queryNone, successMedia: true}, true
	case method == http.MethodGet && isGitHubCollectionPath(path):
		return routePolicy{query: queryPagination, successMedia: true}, true
	default:
		return routePolicy{}, false
	}
}

func isGitHubPointPath(path string) bool {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) == 4 && segments[0] == "v1" && segments[1] == "github" && segments[2] == "owners" {
		return true
	}
	return len(segments) == 6 && segments[0] == "v1" && segments[1] == "github" &&
		segments[2] == "repos" && (segments[5] == "languages" || segments[5] == "") ||
		len(segments) == 5 && segments[0] == "v1" && segments[1] == "github" && segments[2] == "repos"
}

func isGitHubCollectionPath(path string) bool {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) == 5 && segments[0] == "v1" && segments[1] == "github" &&
		segments[2] == "owners" && segments[4] == "repos" {
		return true
	}
	return len(segments) == 6 && segments[0] == "v1" && segments[1] == "github" &&
		segments[2] == "repos" && (segments[5] == "activity" || segments[5] == "tags")
}

func validateRawQuery(rawQuery string, kind queryKind) Code {
	if rawQuery == "" {
		return ""
	}
	allowed := map[string]struct{}{}
	switch kind {
	case queryPagination:
		allowed["limit"] = struct{}{}
		allowed["cursor"] = struct{}{}
	case queryItems:
		allowed["limit"] = struct{}{}
		allowed["cursor"] = struct{}{}
		allowed["category"] = struct{}{}
	}
	seen := make(map[string]struct{})
	for member := range strings.SplitSeq(rawQuery, "&") {
		if member == "" {
			return CodeInvalidRequest
		}
		rawName, rawValue, _ := strings.Cut(member, "=")
		name, err := url.QueryUnescape(rawName)
		if err != nil || !utf8.ValidString(name) {
			return CodeInvalidRequest
		}
		value, err := url.QueryUnescape(rawValue)
		if err != nil || !utf8.ValidString(value) {
			return CodeInvalidRequest
		}
		if _, ok := allowed[name]; !ok {
			return CodeInvalidRequest
		}
		if _, duplicate := seen[name]; duplicate {
			return CodeInvalidRequest
		}
		seen[name] = struct{}{}
		switch name {
		case "cursor":
			if len(value) == 0 || len(value) > 2048 || !isPrintableASCII(value) {
				return CodeInvalidRequest
			}
		case "limit":
			if !isASCIIDecimal(value) {
				return CodeValidationFailed
			}
			limit, err := strconv.ParseUint(value, 10, 64)
			if err != nil || limit < 1 || limit > 100 {
				return CodeValidationFailed
			}
		case "category":
			if !validCategory(value) {
				return CodeValidationFailed
			}
		}
	}
	return ""
}

func validateDeclaredLength(request *http.Request) Code {
	values := request.Header.Values("Content-Length")
	if len(values) > 1 {
		return CodeInvalidRequest
	}
	if len(values) == 1 {
		value := values[0]
		if strings.Contains(value, ",") || !isASCIIDecimal(value) {
			return CodeInvalidRequest
		}
		length, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return CodeInvalidRequest
		}
		if length > uint64(MaxRequestBodyBytes) {
			return CodePayloadTooLarge
		}
	}
	if request.ContentLength > MaxRequestBodyBytes {
		return CodePayloadTooLarge
	}
	if request.ContentLength < -1 {
		return CodeInvalidRequest
	}
	return ""
}

func validateRequestRepresentation(request *http.Request) Code {
	encodings := request.Header.Values("Content-Encoding")
	if len(encodings) > 1 || len(encodings) == 1 &&
		(strings.Contains(encodings[0], ",") || !strings.EqualFold(strings.TrimSpace(encodings[0]), "identity")) {
		return CodeUnsupportedMediaType
	}
	contentTypes := request.Header.Values("Content-Type")
	if len(contentTypes) == 0 {
		if requestHasContent(request) {
			return CodeUnsupportedMediaType
		}
		return ""
	}
	if len(contentTypes) != 1 || parseRequestContentType(contentTypes[0]) == "" {
		return CodeUnsupportedMediaType
	}
	return ""
}

type bufferedRequestBody struct {
	*bufio.Reader
	io.Closer
}

func requestHasContent(request *http.Request) bool {
	if request.ContentLength > 0 {
		return true
	}
	if request.Body == nil || request.Body == http.NoBody {
		return false
	}
	body := request.Body
	buffered := bufio.NewReaderSize(body, 1)
	request.Body = &bufferedRequestBody{Reader: buffered, Closer: body}
	content, _ := buffered.Peek(1)
	return len(content) != 0
}

func parseRequestContentType(value string) string {
	if strings.Contains(value, ",") {
		return ""
	}
	parts, valid := splitSemicolonFields(value)
	if !valid || len(parts) == 0 {
		return ""
	}
	mediaType := strings.ToLower(strings.TrimSpace(parts[0]))
	if mediaType == MediaTypeCBOR {
		if len(parts) == 1 {
			return mediaType
		}
		return ""
	}
	if mediaType != MediaTypeJSON || len(parts) > 2 {
		return ""
	}
	if len(parts) == 1 {
		return mediaType
	}
	name, value, found := strings.Cut(strings.TrimSpace(parts[1]), "=")
	decoded, ok := decodeParameterValue(strings.TrimSpace(value), found)
	if !ok || !strings.EqualFold(strings.TrimSpace(name), "charset") || !strings.EqualFold(decoded, "utf-8") {
		return ""
	}
	return mediaType
}

func isASCIIDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, current := range []byte(value) {
		if current < '0' || current > '9' {
			return false
		}
	}
	return true
}

func isPrintableASCII(value string) bool {
	for _, current := range []byte(value) {
		if current < 0x21 || current > 0x7e {
			return false
		}
	}
	return true
}

func validCategory(value string) bool {
	switch value {
	case "electronics", "tools", "accessories", "robotics", "power", "components":
		return true
	default:
		return false
	}
}

// WriteProblem serializes a safe portable problem without reflecting parser,
// dependency, or provider detail.
func WriteProblem(writer http.ResponseWriter, request *http.Request, code Code, issues ...Issue) {
	problem := NewProblem(code, issues...)
	accept := originalAccept(request.Context())
	if accept == "" {
		accept = AcceptHeader(request.Header)
	}
	mediaType := NegotiateProblem(accept)
	writer.Header().Set("Content-Type", mediaType)
	writer.WriteHeader(problem.Status)
	var err error
	if strings.HasPrefix(mediaType, MediaTypeCBOR) {
		err = marshalCBOR(writer, problem)
	} else {
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(problem)
	}
	if err != nil && !errors.Is(err, io.ErrClosedPipe) {
		panic(err)
	}
}
