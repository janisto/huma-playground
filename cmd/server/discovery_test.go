package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/portable"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

const catalogPath = "/.well-known/api-catalog"

type unreadBody struct{}

func (unreadBody) Read([]byte) (int, error) { panic("discovery read request body") }
func (unreadBody) Close() error             { return nil }

type neverVerifier struct{}

func (neverVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	panic("discovery invoked authentication")
}

type neverProfileStore struct{}

func (neverProfileStore) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	panic("discovery created profile")
}

func (neverProfileStore) Get(context.Context, string) (*profilesvc.Profile, error) {
	panic("discovery read profile")
}

func (neverProfileStore) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	panic("discovery updated profile")
}

func (neverProfileStore) Delete(context.Context, string) error {
	panic("discovery deleted profile")
}

func discoveryApplication(t *testing.T) http.Handler {
	t.Helper()
	cfg := testConfig(t)
	cfg.CORSOrigins = []string{"https://client.example"}
	github, err := githubsvc.NewClient(
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			panic("discovery called GitHub")
		})},
	)
	if err != nil {
		t.Fatal(err)
	}
	router := newRouter(
		cfg,
		dependencies{verifier: neverVerifier{}, profiles: neverProfileStore{}, github: github},
		zap.NewNop(),
	)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Link", `</schema.json>; rel="describedby"; type="application/schema+json"`)
		router.ServeHTTP(writer, request)
	})
}

func discoveryRequest(
	t *testing.T,
	e http.Handler,
	method, target string,
	accept ...string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	req.Body = unreadBody{}
	req.ContentLength = 2_000_000 // Body-free routes must ignore content, even above the write limit.
	req.Header.Set("Origin", "https://client.example")
	req.Header.Set("X-Request-ID", "discovery-test")
	req.Header.Set("Authorization", "Bearer discovery-must-not-authenticate")
	for _, value := range accept {
		req.Header.Add("Accept", value)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func assertDiscoveryLink(t *testing.T, header http.Header, target, relation, contentType string) {
	t.Helper()
	for _, value := range header.Values("Link") {
		for link := range strings.SplitSeq(value, ",") {
			uri, parameters, ok := strings.Cut(strings.TrimSpace(link), ">")
			if !ok || uri != "<"+target {
				continue
			}
			_, params, err := mime.ParseMediaType("application/link" + parameters)
			if err == nil && slices.Contains(strings.Fields(params["rel"]), relation) && params["type"] == contentType {
				return
			}
		}
	}
	t.Fatalf("missing typed link %q/%q/%q in %v", target, relation, contentType, header.Values("Link"))
}

func assertDiscoveryHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range map[string]string{
		"Cache-Control": "no-store", "X-Request-ID": "discovery-test", "X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY", "Referrer-Policy": "strict-origin-when-cross-origin",
		"Access-Control-Allow-Origin": "https://client.example",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	vary := strings.Join(rec.Header().Values("Vary"), ",")
	for _, name := range []string{"Accept", "Origin"} {
		if !strings.Contains(vary, name) {
			t.Fatalf("Vary = %q, missing %q", vary, name)
		}
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Expose-Headers"), "Link") {
		t.Fatal("browser cannot read discovery links")
	}
	assertDiscoveryLink(t, rec.Header(), "/schema.json", "describedby", "application/schema+json")
}

func TestCatalogResolvesSameBuildDocumentsWithoutDependenciesOrFilesystem(t *testing.T) {
	index, err := os.ReadFile("../../llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile("../../llms-full.txt")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	e := discoveryApplication(t)
	const retrievalURL = "http://127.0.0.1:8080/.well-known/api-catalog"
	get := discoveryRequest(t, e, http.MethodGet, retrievalURL)
	if get.Code != 200 {
		t.Fatalf("catalog = %d: %s", get.Code, get.Body.String())
	}
	contentType, params, err := mime.ParseMediaType(get.Header().Get("Content-Type"))
	if err != nil || contentType != "application/linkset+json" ||
		params["profile"] != "https://www.rfc-editor.org/info/rfc9727" {
		t.Fatalf("catalog media = %q, err=%v", get.Header().Get("Content-Type"), err)
	}
	assertDiscoveryHeaders(t, get)
	assertDiscoveryLink(t, get.Header(), catalogPath, "api-catalog", "application/linkset+json")
	assertDiscoveryLink(t, get.Header(), "/llms.txt", "describedby", "text/plain")

	var catalog map[string][]map[string]json.RawMessage
	if err = json.Unmarshal(get.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 || len(catalog["linkset"]) != 1 {
		t.Fatalf("unexpected catalog shape: %#v", catalog)
	}
	context := catalog["linkset"][0]
	var anchor string
	if err = json.Unmarshal(context["anchor"], &anchor); err != nil || anchor != "/" {
		t.Fatalf("catalog anchor = %q, err=%v", anchor, err)
	}
	base, err := url.Parse(retrievalURL)
	if err != nil {
		t.Fatal(err)
	}
	for relation, expected := range map[string]map[string]string{
		"service-desc": {"/openapi.json": "application/json"},
		"service-doc":  {"/llms.txt": "text/plain", "/llms-full.txt": "text/plain"},
		"status":       {"/health": "application/json"},
	} {
		var targets []struct {
			Href string `json:"href"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(context[relation], &targets); err != nil || len(targets) != len(expected) {
			t.Fatalf("catalog %s = %s, err=%v", relation, context[relation], err)
		}
		seen := map[string]bool{}
		for _, target := range targets {
			if expected[target.Href] != target.Type || seen[target.Href] {
				t.Fatalf("unexpected/duplicate %s target: %#v", relation, target)
			}
			seen[target.Href] = true
			ref, err := url.Parse(target.Href)
			if err != nil {
				t.Fatal(err)
			}
			resolved := base.ResolveReference(ref)
			if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
				t.Fatalf("catalog target left the origin: %s", resolved)
			}
			response := discoveryRequest(t, e, http.MethodGet, resolved.String(), target.Type)
			actualType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
			if response.Code != 200 || err != nil || actualType != target.Type {
				t.Fatalf("resolved %s = %d/%q, err=%v", resolved, response.Code, actualType, err)
			}
			assertDiscoveryHeaders(t, response)
			want := map[string][]byte{"/llms.txt": index, "/llms-full.txt": guide}[target.Href]
			if want != nil && !bytes.Equal(response.Body.Bytes(), want) {
				t.Fatalf("%s differs from this build's repository document", target.Href)
			}
		}
	}

	head := discoveryRequest(t, e, http.MethodHead, retrievalURL)
	if head.Code != get.Code || head.Body.Len() != 0 {
		t.Fatalf("catalog HEAD = %d, bytes=%d", head.Code, head.Body.Len())
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Cache-Control", "X-Request-ID"} {
		if strings.Join(head.Header().Values(name), ",") != strings.Join(get.Header().Values(name), ",") {
			t.Fatalf("HEAD %s differs from GET", name)
		}
	}
	assertDiscoveryLink(t, head.Header(), catalogPath, "api-catalog", "application/linkset+json")
	assertDiscoveryLink(t, head.Header(), "/llms.txt", "describedby", "text/plain")
	assertDiscoveryHeaders(t, head)
}

func TestHealthDiscoveryLinksCoverJSONAndCBOR(t *testing.T) {
	e := discoveryApplication(t)
	for _, accept := range []string{"application/json", "application/cbor"} {
		rec := discoveryRequest(t, e, http.MethodGet, "/health", accept)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != accept {
			t.Fatalf("health = %d/%q", rec.Code, rec.Header().Get("Content-Type"))
		}
		var body map[string]string
		var err error
		if accept == "application/cbor" {
			err = cbor.Unmarshal(rec.Body.Bytes(), &body)
		} else {
			err = json.Unmarshal(rec.Body.Bytes(), &body)
		}
		if err != nil || len(body) != 1 || body["status"] != "healthy" {
			t.Fatalf("health body changed: %#v, err=%v", body, err)
		}
		assertDiscoveryLink(t, rec.Header(), catalogPath, "api-catalog", "application/linkset+json")
		assertDiscoveryLink(t, rec.Header(), "/openapi.json", "service-desc", "application/json")
		assertDiscoveryHeaders(t, rec)
	}
}

func TestDocumentationAdvertisesIndexAndPreservesOtherDescribedbyTargets(t *testing.T) {
	e := discoveryApplication(t)
	for _, path := range []string{catalogPath, "/openapi.json", "/llms-full.txt"} {
		rec := discoveryRequest(t, e, http.MethodGet, path)
		if rec.Code != 200 {
			t.Fatalf("%s = %d", path, rec.Code)
		}
		assertDiscoveryLink(t, rec.Header(), "/llms.txt", "describedby", "text/plain")
		assertDiscoveryLink(t, rec.Header(), "/schema.json", "describedby", "application/schema+json")
		index := discoveryRequest(t, e, http.MethodGet, "/llms.txt", "text/plain")
		if index.Code != 200 || index.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatalf("index is not public plain text: %d/%q", index.Code, index.Header().Get("Content-Type"))
		}
	}
}

func TestDocumentationNegotiationMatchesMediaParametersAndQuality(t *testing.T) {
	e := discoveryApplication(t)
	for _, test := range []struct {
		path   string
		accept []string
		status int
	}{
		{catalogPath, []string{""}, 406},
		{catalogPath, []string{"application/linkset+json"}, 200},
		{catalogPath, []string{`application/linkset+json; profile="https://www.rfc-editor.org/info/rfc9727"`}, 200},
		{catalogPath, []string{`application/linkset+json;q=1;profile="https://www.rfc-editor.org/info/rfc9727"`}, 200},
		{catalogPath, []string{"application/json"}, 406},
		{catalogPath, []string{"application/cbor"}, 406},
		{catalogPath, []string{`application/linkset+json;profile="https://www.rfc-editor.org/info/RFC9727"`}, 406},
		{catalogPath, []string{`application/linkset+json;q=1;profile=other`}, 406},
		{catalogPath, []string{"*/*;q=1", "application/linkset+json;q=0"}, 406},
		{catalogPath, []string{"application/json;q=1", "application/linkset+json;q=0.2"}, 200},
		{catalogPath, []string{"application/linkset+json;q=bogus"}, 406},
		{"/llms.txt", []string{""}, 406},
		{"/llms.txt", []string{"text/plain; charset=UTF-8"}, 200},
		{"/llms.txt", []string{"text/plain;q=1;charset=UTF-8"}, 200},
		{"/llms.txt", []string{"text/plain; charset=iso-8859-1"}, 406},
		{"/llms.txt", []string{"text/plain;q=0, */*;q=1"}, 406},
		{"/llms-full.txt", []string{"application/cbor"}, 406},
		{"/llms-full.txt", []string{"text/html"}, 406},
		{"/llms-full.txt", []string{"application/json", "text/plain"}, 200},
	} {
		t.Run(test.path+strings.Join(test.accept, ","), func(t *testing.T) {
			rec := discoveryRequest(t, e, http.MethodGet, test.path, test.accept...)
			if rec.Code != test.status {
				t.Fatalf("response = %d: %s", rec.Code, rec.Body.String())
			}
			assertDiscoveryHeaders(t, rec)
			if test.status == 406 {
				assertDiscoveryProblem(t, rec, "not_acceptable")
			}
		})
	}
}

func assertDiscoveryProblem(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("documentation error is not JSON: %q", rec.Header().Get("Content-Type"))
	}
	var problem portable.ProblemError
	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&problem,
	); err != nil || problem.Status != rec.Code ||
		string(problem.Code) != code {
		t.Fatalf("documentation problem = %#v, err=%v", problem, err)
	}
}

func TestDocumentationRejectsQueriesAndMethodsWithJSONProblems(t *testing.T) {
	e := discoveryApplication(t)
	for _, path := range []string{catalogPath, "/llms.txt", "/llms-full.txt", "/openapi.json"} {
		for _, query := range []string{"?unknown=1", "?unknown=1&unknown=2", "?bad=%ZZ", "?x=%FF", "?x=1;y=2"} {
			rec := discoveryRequest(t, e, http.MethodGet, path+query, "*/*;q=0.8,application/cbor;q=1")
			if rec.Code != 400 {
				t.Fatalf("%s = %d", path+query, rec.Code)
			}
			assertDiscoveryProblem(t, rec, "invalid_request")
			if path != catalogPath {
				continue
			}
			head := discoveryRequest(t, e, http.MethodHead, path+query, "*/*;q=0.8,application/cbor;q=1")
			if head.Code != 400 || head.Body.Len() != 0 ||
				head.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf(
					"HEAD %s = %d, bytes=%d, media=%q",
					path+query,
					head.Code,
					head.Body.Len(),
					head.Header().Get("Content-Type"),
				)
			}
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			rec := discoveryRequest(t, e, method, path, "application/cbor")
			if rec.Code != 405 {
				t.Fatalf("%s %s = %d", method, path, rec.Code)
			}
			assertDiscoveryProblem(t, rec, "method_not_allowed")
			var allow []string
			for name := range strings.SplitSeq(rec.Header().Get("Allow"), ",") {
				allow = append(allow, strings.TrimSpace(name))
			}
			slices.Sort(allow)
			wantAllow := []string{http.MethodGet}
			if path == catalogPath {
				wantAllow = append(wantAllow, http.MethodHead)
			}
			if !slices.Equal(allow, wantAllow) {
				t.Fatalf("untruthful Allow: %v", allow)
			}
		}
		rec := discoveryRequest(t, e, http.MethodGet, path, "application/cbor")
		if rec.Code != 406 {
			t.Fatalf("GET %s accepted CBOR: %d", path, rec.Code)
		}
		assertDiscoveryProblem(t, rec, "not_acceptable")
		head := discoveryRequest(t, e, http.MethodHead, path, "application/cbor")
		wantHead := http.StatusMethodNotAllowed
		if path == catalogPath {
			wantHead = http.StatusNotAcceptable
		}
		if head.Code != wantHead || head.Body.Len() != 0 ||
			head.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("HEAD negotiation failure = %d, bytes=%d", head.Code, head.Body.Len())
		}
	}
}

func TestDocumentationDoesNotExposeRepositoryFiles(t *testing.T) {
	e := discoveryApplication(t)
	for _, path := range []string{"/.env", "/README.md", "/documentation.go", "/api-docs", "/api-docs/swagger.json", "/internal/platform/auth/middleware.go"} {
		rec := discoveryRequest(t, e, http.MethodGet, path)
		if rec.Code != 404 {
			t.Fatalf("unexpected repository exposure at %s: %d", path, rec.Code)
		}
	}
}
