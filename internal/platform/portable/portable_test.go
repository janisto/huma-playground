package portable

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestParseStrictJSONAcceptsOneRFC8259Document(t *testing.T) {
	value, err := ParseStrictJSON(
		[]byte(" \n{\"text\":\"\\ud800\\udc00\",\"number\":-1.25e+2,\"array\":[true,false,null]} \t"),
	)
	if err != nil {
		t.Fatalf("parse valid JSON: %v", err)
	}
	object, isObject := value.(map[string]any)
	number, isNumber := object["number"].(json.Number)
	if !isObject || !isNumber || object["text"] != "𐀀" || number.String() != "-1.25e+2" {
		t.Fatalf("value=%#v", value)
	}
}

func TestParseStrictJSONRejectsAmbiguousMalformedAndNonUnicodeDocuments(t *testing.T) {
	tests := map[string][]byte{
		"empty":              {},
		"whitespace":         []byte(" \n"),
		"BOM":                {0xef, 0xbb, 0xbf, '{', '}'},
		"invalid UTF-8":      {'"', 0xff, '"'},
		"duplicate root":     []byte("{\"x\":1,\"x\":2}"),
		"duplicate nested":   []byte("{\"x\":{\"y\":1,\"y\":2}}"),
		"trailing document":  []byte("{}[]"),
		"trailing comma":     []byte("{\"x\":1,}"),
		"leading zero":       []byte("01"),
		"non-finite":         []byte("NaN"),
		"lone high":          []byte("\"\\ud800\""),
		"lone low":           []byte("\"\\udc00\""),
		"bad surrogate pair": []byte("\"\\ud800\\u0041\""),
		"bad escape":         []byte("\"\\x20\""),
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseStrictJSON(document); err == nil {
				t.Fatalf("invalid document accepted: %q", document)
			}
		})
	}
}

func TestParseStrictJSONBoundsContainerNesting(t *testing.T) {
	const expectedMaxJSONNestedLevels = 32
	tests := []struct {
		name     string
		open     string
		close    string
		accepted bool
		depth    int
	}{
		{name: "array at limit", open: "[", close: "]", depth: expectedMaxJSONNestedLevels, accepted: true},
		{name: "array over limit", open: "[", close: "]", depth: expectedMaxJSONNestedLevels + 1},
		{name: "object at limit", open: `{"value":`, close: "}", depth: expectedMaxJSONNestedLevels, accepted: true},
		{name: "object over limit", open: `{"value":`, close: "}", depth: expectedMaxJSONNestedLevels + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := strings.Repeat(test.open, test.depth) + "0" + strings.Repeat(test.close, test.depth)
			_, err := ParseStrictJSON([]byte(document))
			if test.accepted && err != nil {
				t.Fatalf("depth %d rejected: %v", test.depth, err)
			}
			if !test.accepted && err == nil {
				t.Fatalf("depth %d accepted", test.depth)
			}
		})
	}
}

func TestStrictJSONUnmarshalDoesNotCoerceTypes(t *testing.T) {
	var value struct{ Count int }
	if err := StrictJSONUnmarshal([]byte("{\"count\":\"1\"}"), &value); err == nil {
		t.Fatal("string was coerced to integer")
	}
	if err := StrictJSONUnmarshal([]byte("{\"count\":1}"), &value); err != nil || value.Count != 1 {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}

func TestStrictCBORDecoderRejectsDuplicateKeysTagsAndIndefiniteValues(t *testing.T) {
	unmarshal := Formats()[MediaTypeCBOR].Unmarshal
	tests := map[string][]byte{
		"duplicate keys": {0xa2, 0x61, 'x', 0x01, 0x61, 'x', 0x02},
		"tag":            {0xc0, 0x61, 'x'},
		"indefinite":     {0xbf, 0x61, 'x', 0x01, 0xff},
		"invalid UTF-8":  {0x61, 0xff},
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			var value any
			if err := unmarshal(document, &value); err == nil {
				t.Fatalf("invalid CBOR accepted: %x value=%#v", document, value)
			}
		})
	}
	document, err := cbor.Marshal(math.Inf(1))
	if err != nil {
		t.Fatal(err)
	}
	var number float64
	if err := unmarshal(document, &number); err != nil || !math.IsInf(number, 1) {
		t.Fatalf("well-formed non-finite CBOR is a schema concern, value=%v err=%v", number, err)
	}
}

func TestNegotiationUsesSpecificityQualityAndDeterministicTies(t *testing.T) {
	tests := []struct {
		name, accept, want string
		jsonOnly           bool
		ok                 bool
	}{
		{name: "missing defaults JSON", want: MediaTypeJSON, ok: true},
		{name: "wildcard defaults JSON", accept: "*/*", want: MediaTypeJSON, ok: true},
		{name: "JSON CBOR tie uses JSON", accept: "application/cbor, application/json", want: MediaTypeJSON, ok: true},
		{
			name:   "higher CBOR quality",
			accept: "application/json;q=0.5, application/cbor;q=0.9",
			want:   MediaTypeCBOR,
			ok:     true,
		},
		{name: "charset only", accept: "application/json; charset=UTF-8", want: MediaTypeJSONUTF8, ok: true},
		{name: "exact exclusion controls wildcard", accept: "application/json;q=0, */*;q=1", ok: false},
		{
			name:   "malformed ignored beside valid",
			accept: "text/plain;q=2, application/json",
			want:   MediaTypeJSON,
			ok:     true,
		},
		{name: "problem type is not success", accept: MediaTypeProblemJSON, ok: false},
		{name: "CBOR forbidden for discovery", accept: MediaTypeCBOR, jsonOnly: true, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NegotiateSuccess(test.accept, test.jsonOnly)
			if got != test.want || ok != test.ok {
				t.Fatalf("NegotiateSuccess(%q)=(%q,%t), want (%q,%t)", test.accept, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestProblemNegotiationAndTaxonomyAreExact(t *testing.T) {
	tests := map[string]string{
		"":                                       MediaTypeProblemJSON,
		"application/problem+json":               MediaTypeProblemJSON,
		"application/problem+json;charset=utf-8": MediaTypeProblemJSON + "; charset=utf-8",
		"application/cbor":                       MediaTypeCBOR,
		"application/json":                       MediaTypeProblemJSON,
		"application/problem+cbor":               MediaTypeProblemJSON,
		"text/plain":                             MediaTypeProblemJSON,
	}
	for accept, want := range tests {
		if got := NegotiateProblem(accept); got != want {
			t.Errorf("NegotiateProblem(%q)=%q want=%q", accept, got, want)
		}
	}
	for code, definition := range problemDefinitions {
		problem := NewProblem(code)
		if problem.Code != code || problem.Status != definition.Status || problem.Title != definition.Title ||
			problem.Detail != definition.Detail || len(problem.Errors) != 0 {
			t.Errorf("problem %q=%#v definition=%#v", code, problem, definition)
		}
	}
}

func TestValidationIssueNormalizationNeverReflectsUnknownInput(t *testing.T) {
	known := normalizeValidationIssue("body.contactEmail")
	if known.Source == nil || known.Source.Pointer == nil || *known.Source.Pointer != "/contactEmail" {
		t.Fatalf("known issue=%#v", known)
	}
	unknown := normalizeValidationIssue("body.attacker-secret")
	encoded, err := json.Marshal(unknown)
	if err != nil || strings.Contains(string(encoded), "attacker") || unknown.Source != nil {
		t.Fatalf("unknown issue=%s err=%v", encoded, err)
	}
	issues := make([]Issue, 40)
	for index := range issues {
		issues[index] = Issue{Detail: "Invalid request value"}
	}
	if bounded := boundedIssues(
		issues,
	); len(bounded) != 32 ||
		bounded[31].Detail != "Additional validation errors omitted" {
		t.Fatalf("bounded issues=%#v", bounded)
	}
}

func TestPortableFieldBoundariesAndCanonicalization(t *testing.T) {
	if !ValidBoundedName("María\u202fJosé") || !ValidBoundedName(strings.Repeat("𐀀", 100)) {
		t.Fatal("valid bounded names rejected")
	}
	for _, value := range []string{"", " Ada", "Ada ", "A\u0000da", strings.Repeat("a", 101), "\u00a0Ada"} {
		if ValidBoundedName(value) {
			t.Errorf("invalid bounded name accepted: %q", value)
		}
	}
	email, ok := NormalizeContactEmail("\tAda+tag@EXAMPLE.COM\n")
	if !ok || email != "Ada+tag@example.com" {
		t.Fatalf("email=%q ok=%t", email, ok)
	}
	for _, value := range []string{
		"a@example", ".a@example.com", "a.@example.com", "a..b@example.com",
		"a@-example.com", "a@example-.com", "a@éxample.com", strings.Repeat("a", 65) + "@example.com",
		"a@" + strings.Repeat("a", 64) + ".com", strings.Repeat("a", 245) + "@example.com",
	} {
		if _, valid := NormalizeContactEmail(value); valid {
			t.Errorf("invalid email accepted: %q", value)
		}
	}
	phone, ok := NormalizePhoneNumber("\r+358401234567 ")
	if !ok || phone != "+358401234567" {
		t.Fatalf("phone=%q ok=%t", phone, ok)
	}
	for _, value := range []string{"+01234567", "358401234567", "+123456", "+1234567890123456", "+123 4567"} {
		if _, valid := NormalizePhoneNumber(value); valid {
			t.Errorf("invalid phone accepted: %q", value)
		}
	}
}

func TestRequestPolicyRejectsClosedQueryMediaAndNegotiationBeforeHandler(t *testing.T) {
	var calls int
	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		response.Header().Set("Content-Type", request.Header.Get("Accept"))
		response.WriteHeader(http.StatusOK)
	})
	handler := RequestPolicy("/v1")(next)
	tests := []struct {
		name, method, target, contentType, encoding, accept string
		body                                                string
		want                                                int
		code                                                Code
	}{
		{
			name:   "unknown health query",
			method: "GET",
			target: "/health?x=1",
			accept: MediaTypeJSON,
			want:   400,
			code:   CodeInvalidRequest,
		},
		{
			name:   "blank query member",
			method: "GET",
			target: "/v1/items?limit=1&",
			accept: MediaTypeJSON,
			want:   400,
			code:   CodeInvalidRequest,
		},
		{
			name:   "repeated item query",
			method: "GET",
			target: "/v1/items?limit=1&limit=2",
			accept: MediaTypeJSON,
			want:   400,
			code:   CodeInvalidRequest,
		},
		{
			name:   "bad item limit",
			method: "GET",
			target: "/v1/items?limit=-1",
			accept: MediaTypeJSON,
			want:   422,
			code:   CodeValidationFailed,
		},
		{
			name:   "unsupported success",
			method: "GET",
			target: "/v1/items",
			accept: "text/plain",
			want:   406,
			code:   CodeNotAcceptable,
		},
		{
			name:   "missing media on content",
			method: "POST",
			target: "/v1/hello",
			body:   "{}",
			accept: MediaTypeJSON,
			want:   415,
			code:   CodeUnsupportedMediaType,
		},
		{
			name:        "unsupported media",
			method:      "POST",
			target:      "/v1/hello",
			body:        "{}",
			contentType: "text/plain",
			accept:      MediaTypeJSON,
			want:        415,
			code:        CodeUnsupportedMediaType,
		},
		{
			name:        "ambiguous media",
			method:      "POST",
			target:      "/v1/hello",
			body:        "{}",
			contentType: "application/json, application/cbor",
			accept:      MediaTypeJSON,
			want:        415,
			code:        CodeUnsupportedMediaType,
		},
		{
			name:        "compressed",
			method:      "POST",
			target:      "/v1/hello",
			body:        "{}",
			contentType: MediaTypeJSON,
			encoding:    "gzip",
			accept:      MediaTypeJSON,
			want:        415,
			code:        CodeUnsupportedMediaType,
		},
		{
			name:        "accepted JSON charset",
			method:      "POST",
			target:      "/v1/hello",
			body:        "{}",
			contentType: "Application/JSON; Charset=\"UTF-8\"",
			accept:      MediaTypeJSON,
			want:        200,
		},
		{
			name:   "bodyless delete ignores Accept",
			method: "DELETE",
			target: "/v1/profile",
			accept: "text/plain",
			want:   200,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls = 0
			request := httptest.NewRequestWithContext(
				t.Context(), test.method, test.target, strings.NewReader(test.body),
			)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			if test.encoding != "" {
				request.Header.Set("Content-Encoding", test.encoding)
			}
			if test.accept != "" {
				request.Header.Set("Accept", test.accept)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == 200 {
				if calls != 1 {
					t.Fatalf("handler calls=%d", calls)
				}
				return
			}
			if calls != 0 {
				t.Fatalf("handler called %d times", calls)
			}
			var problem ProblemError
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil || problem.Code != test.code {
				t.Fatalf("problem=%#v err=%v body=%s", problem, err, response.Body.String())
			}
		})
	}
}

func TestRequestPolicyRejectsInvalidDeclaredLengthsBeforeHandler(t *testing.T) {
	tests := []struct {
		name          string
		headerValues  []string
		contentLength int64
		code          Code
	}{
		{name: "negative field", headerValues: []string{"-1"}, contentLength: 2, code: CodeInvalidRequest},
		{name: "signed field", headerValues: []string{"+2"}, contentLength: 2, code: CodeInvalidRequest},
		{name: "whitespace field", headerValues: []string{" 2"}, contentLength: 2, code: CodeInvalidRequest},
		{name: "non-decimal field", headerValues: []string{"2e0"}, contentLength: 2, code: CodeInvalidRequest},
		{
			name: "overflowed field", headerValues: []string{"18446744073709551616"},
			contentLength: 2, code: CodeInvalidRequest,
		},
		{name: "comma field", headerValues: []string{"1,1"}, contentLength: 2, code: CodeInvalidRequest},
		{name: "conflicting fields", headerValues: []string{"1", "2"}, contentLength: 2, code: CodeInvalidRequest},
		{name: "invalid native length", contentLength: -2, code: CodeInvalidRequest},
		{name: "native overflow", contentLength: MaxRequestBodyBytes + 1, code: CodePayloadTooLarge},
		{
			name: "declared overflow", headerValues: []string{"1000001"},
			contentLength: 2, code: CodePayloadTooLarge,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			handler := RequestPolicy("/v1")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls++
			}))
			request := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/v1/hello", strings.NewReader("{}"),
			)
			request.ContentLength = test.contentLength
			request.Header.Set("Accept", MediaTypeJSON)
			request.Header.Set("Content-Type", MediaTypeJSON)
			request.Header.Del("Content-Length")
			for _, value := range test.headerValues {
				request.Header.Add("Content-Length", value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if calls != 0 {
				t.Fatalf("handler calls=%d", calls)
			}
			var problem ProblemError
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil || problem.Code != test.code {
				t.Fatalf("status=%d problem=%#v err=%v body=%s", response.Code, problem, err, response.Body.String())
			}
		})
	}
}

func TestFormatsEmitNoHTMLEscapingAndValidCBOR(t *testing.T) {
	value := map[string]any{"text": "<>&", "count": uint64(9_007_199_254_740_991)}
	var jsonBuffer bytes.Buffer
	if err := Formats()[MediaTypeJSON].Marshal(&jsonBuffer, value); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonBuffer.String(), "\\u003c") {
		t.Fatalf("JSON HTML-escaped: %s", jsonBuffer.String())
	}
	var cborBuffer bytes.Buffer
	if err := Formats()[MediaTypeCBOR].Marshal(&cborBuffer, value); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := cbor.Unmarshal(cborBuffer.Bytes(), &decoded); err != nil || decoded["text"] != "<>&" {
		t.Fatalf("CBOR=%#v err=%v", decoded, err)
	}
}
