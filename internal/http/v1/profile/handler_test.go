package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/portable"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type profileVerifier struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (verifier *profileVerifier) Verify(_ context.Context, token string) (*auth.FirebaseUser, error) {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	verifier.calls++
	if verifier.err != nil {
		return nil, verifier.err
	}
	switch token {
	case "token-a":
		return &auth.FirebaseUser{UID: "principal-a"}, nil
	case "token-b":
		return &auth.FirebaseUser{UID: "principal-b"}, nil
	default:
		return nil, auth.ErrInvalidToken
	}
}

func (verifier *profileVerifier) callCount() int {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	return verifier.calls
}

type memoryProfileStore struct {
	mu         sync.Mutex
	profiles   map[string]*profilesvc.Profile
	now        time.Time
	err        error
	calls      int
	writeCount int
	principals []string
}

type uncertainProfileStore struct {
	*memoryProfileStore
}

func (store *uncertainProfileStore) Update(
	ctx context.Context,
	principal string,
	params profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	if _, err := store.memoryProfileStore.Update(ctx, principal, params); err != nil {
		return nil, err
	}
	return nil, profilesvc.ErrUnavailable
}

func newMemoryProfileStore() *memoryProfileStore {
	return &memoryProfileStore{
		profiles: make(map[string]*profilesvc.Profile),
		now:      time.Date(2026, 7, 30, 12, 0, 0, 987_654_321, time.UTC),
	}
}

func cloneProfile(profile *profilesvc.Profile) *profilesvc.Profile {
	if profile == nil {
		return nil
	}
	clone := *profile
	return &clone
}

func (store *memoryProfileStore) record(principal string) error {
	store.calls++
	store.principals = append(store.principals, principal)
	return store.err
}

func (store *memoryProfileStore) Create(
	_ context.Context,
	principal string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.record(principal); err != nil {
		return nil, err
	}
	if _, exists := store.profiles[principal]; exists {
		return nil, profilesvc.ErrAlreadyExists
	}
	now := store.now.UTC().Truncate(time.Millisecond)
	profile := &profilesvc.Profile{
		ID: principal, FirstName: params.FirstName, LastName: params.LastName,
		ContactEmail: params.ContactEmail, PhoneNumber: params.PhoneNumber,
		MarketingOptIn: params.MarketingOptIn, TermsAccepted: params.TermsAccepted,
		CreatedAt: now, UpdatedAt: now,
	}
	store.profiles[principal] = cloneProfile(profile)
	store.writeCount++
	return cloneProfile(profile), nil
}

func (store *memoryProfileStore) Get(_ context.Context, principal string) (*profilesvc.Profile, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.record(principal); err != nil {
		return nil, err
	}
	profile, exists := store.profiles[principal]
	if !exists {
		return nil, profilesvc.ErrNotFound
	}
	return cloneProfile(profile), nil
}

func (store *memoryProfileStore) Update(
	_ context.Context,
	principal string,
	params profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.record(principal); err != nil {
		return nil, err
	}
	current, exists := store.profiles[principal]
	if !exists {
		return nil, profilesvc.ErrNotFound
	}
	next := cloneProfile(current)
	changed := false
	if params.FirstName != nil && *params.FirstName != next.FirstName {
		next.FirstName, changed = *params.FirstName, true
	}
	if params.LastName != nil && *params.LastName != next.LastName {
		next.LastName, changed = *params.LastName, true
	}
	if params.ContactEmail != nil && *params.ContactEmail != next.ContactEmail {
		next.ContactEmail, changed = *params.ContactEmail, true
	}
	if params.PhoneNumber != nil && *params.PhoneNumber != next.PhoneNumber {
		next.PhoneNumber, changed = *params.PhoneNumber, true
	}
	if params.MarketingOptIn != nil && *params.MarketingOptIn != next.MarketingOptIn {
		next.MarketingOptIn, changed = *params.MarketingOptIn, true
	}
	if !changed {
		return cloneProfile(current), nil
	}
	now := store.now.UTC().Truncate(time.Millisecond)
	if !now.After(next.UpdatedAt) {
		now = next.UpdatedAt.Add(time.Millisecond)
	}
	next.UpdatedAt = now
	store.profiles[principal] = cloneProfile(next)
	store.writeCount++
	return next, nil
}

func (store *memoryProfileStore) Delete(_ context.Context, principal string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.record(principal); err != nil {
		return err
	}
	if _, exists := store.profiles[principal]; !exists {
		return profilesvc.ErrNotFound
	}
	delete(store.profiles, principal)
	store.writeCount++
	return nil
}

func (store *memoryProfileStore) counts() (calls, writes int) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.calls, store.writeCount
}

func newProfileTestRouter(t *testing.T, store profilesvc.Store, verifier auth.Verifier) http.Handler {
	t.Helper()
	portable.ConfigureHuma()
	config := huma.DefaultConfig("Profile test", "test")
	config.DocsPath = ""
	config.OpenAPIPath = ""
	config.SchemasPath = ""
	config.CreateHooks = nil
	config.Transformers = nil
	config.Formats = portable.Formats()
	config.DefaultFormat = portable.MediaTypeJSON
	config.NoFormatFallback = true
	router := chi.NewRouter()
	router.Use(portable.RequestPolicy("/v1"))
	api := humachi.New(router, config)
	group := huma.NewGroup(api, "/v1")
	group.UseMiddleware(auth.NewAuthMiddleware(api, verifier))
	Register(group, "/v1", store)
	return router
}

func profileRequest(
	t *testing.T,
	router http.Handler,
	method string,
	body io.Reader,
	token string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "/v1/profile", body)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeProfile(t *testing.T, response *httptest.ResponseRecorder) Profile {
	t.Helper()
	var profile Profile
	if err := json.Unmarshal(response.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v; body=%s", err, response.Body.String())
	}
	return profile
}

func decodeProblem(t *testing.T, response *httptest.ResponseRecorder) portable.ProblemError {
	t.Helper()
	var problem portable.ProblemError
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v; body=%s", err, response.Body.String())
	}
	return problem
}

const validCreate = `{"firstName":"Ada","lastName":"Lovelace","contactEmail":" Ada@EXAMPLE.COM \t","phoneNumber":"\n+358401234567 ","termsAccepted":true}`

func TestProfileCRUDLifecycleAndNormalization(t *testing.T) {
	store := newMemoryProfileStore()
	verifier := &profileVerifier{}
	router := newProfileTestRouter(t, store, verifier)

	createdResponse := profileRequest(t, router, http.MethodPost, strings.NewReader(validCreate), "token-a")
	if createdResponse.Code != http.StatusCreated || createdResponse.Header().Get("Location") != "/v1/profile" ||
		createdResponse.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("create status=%d Location=%q Content-Type=%q body=%s",
			createdResponse.Code, createdResponse.Header().Get("Location"),
			createdResponse.Header().Get("Content-Type"), createdResponse.Body.String())
	}
	created := decodeProfile(t, createdResponse)
	if created.ID != "principal-a" || created.ContactEmail != "Ada@example.com" ||
		created.PhoneNumber != "+358401234567" || created.MarketingOptIn || !created.TermsAccepted ||
		created.CreatedAt.Format("2006-01-02T15:04:05.000Z") != "2026-07-30T12:00:00.987Z" ||
		!created.CreatedAt.Equal(created.UpdatedAt.Time) {
		t.Fatalf("created profile = %#v", created)
	}
	var createObject map[string]any
	if err := json.Unmarshal(
		createdResponse.Body.Bytes(),
		&createObject,
	); err != nil || len(createObject) != 9 ||
		createObject["$schema"] != nil {
		t.Fatalf("create envelope not closed: %#v err=%v", createObject, err)
	}

	getResponse := profileRequest(t, router, http.MethodGet, nil, "token-a")
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}
	if got := decodeProfile(t, getResponse); got != created {
		t.Fatalf("get=%#v want=%#v", got, created)
	}

	noOp := profileRequest(
		t,
		router,
		http.MethodPatch,
		strings.NewReader(`{"contactEmail":" Ada@EXAMPLE.COM "}`),
		"token-a",
	)
	if noOp.Code != http.StatusOK {
		t.Fatalf("no-op status=%d body=%s", noOp.Code, noOp.Body.String())
	}
	if got := decodeProfile(t, noOp); !got.UpdatedAt.Equal(created.UpdatedAt.Time) {
		t.Fatalf("no-op updatedAt=%s want=%s", got.UpdatedAt.Time, created.UpdatedAt.Time)
	}
	if _, writes := store.counts(); writes != 1 {
		t.Fatalf("no-op committed a write; writes=%d", writes)
	}

	store.mu.Lock()
	store.now = created.UpdatedAt.Add(-time.Hour)
	store.mu.Unlock()
	updatedResponse := profileRequest(
		t,
		router,
		http.MethodPatch,
		strings.NewReader(`{"marketingOptIn":true}`),
		"token-a",
	)
	updated := decodeProfile(t, updatedResponse)
	if updatedResponse.Code != http.StatusOK || !updated.MarketingOptIn ||
		!updated.CreatedAt.Equal(created.CreatedAt.Time) ||
		!updated.UpdatedAt.Equal(created.UpdatedAt.Add(time.Millisecond)) {
		t.Fatalf("updated status=%d profile=%#v", updatedResponse.Code, updated)
	}

	deleteResponse := profileRequest(t, router, http.MethodDelete, nil, "token-a")
	if deleteResponse.Code != http.StatusNoContent || deleteResponse.Body.Len() != 0 ||
		deleteResponse.Header().Get("Content-Type") != "" {
		t.Fatalf(
			"delete status=%d content-type=%q body=%q",
			deleteResponse.Code,
			deleteResponse.Header().Get("Content-Type"),
			deleteResponse.Body.String(),
		)
	}
	missing := profileRequest(t, router, http.MethodGet, nil, "token-a")
	if missing.Code != http.StatusNotFound || decodeProblem(t, missing).Code != portable.CodeProfileNotFound {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
	if calls, writes := store.counts(); calls != 6 || writes != 3 {
		t.Fatalf("store calls=%d writes=%d", calls, writes)
	}
}

func TestProfileOwnershipUsesOnlyVerifiedPrincipal(t *testing.T) {
	store := newMemoryProfileStore()
	router := newProfileTestRouter(t, store, &profileVerifier{})
	bodyA := strings.Replace(validCreate, `"Ada"`, `"Alice"`, 1)
	bodyB := strings.Replace(validCreate, `"Ada"`, `"Bob"`, 1)
	if response := profileRequest(
		t,
		router,
		http.MethodPost,
		strings.NewReader(bodyA),
		"token-a",
	); response.Code != 201 {
		t.Fatalf("create A status=%d body=%s", response.Code, response.Body.String())
	}
	if response := profileRequest(
		t,
		router,
		http.MethodPost,
		strings.NewReader(bodyB),
		"token-b",
	); response.Code != 201 {
		t.Fatalf("create B status=%d body=%s", response.Code, response.Body.String())
	}
	if got := decodeProfile(
		t,
		profileRequest(t, router, http.MethodGet, nil, "token-a"),
	); got.ID != "principal-a" ||
		got.FirstName != "Alice" {
		t.Fatalf("principal A got %#v", got)
	}
	if got := decodeProfile(
		t,
		profileRequest(t, router, http.MethodGet, nil, "token-b"),
	); got.ID != "principal-b" ||
		got.FirstName != "Bob" {
		t.Fatalf("principal B got %#v", got)
	}
}

func TestProfileValidationHasNoPersistenceSideEffects(t *testing.T) {
	tests := []struct {
		name, method, document string
		status                 int
	}{
		{
			name:     "unknown create member",
			method:   "POST",
			document: strings.TrimSuffix(validCreate, "}") + `,"id":"other"}`,
			status:   422,
		},
		{
			name:     "duplicate member",
			method:   "POST",
			document: strings.Replace(validCreate, `"firstName":"Ada"`, `"firstName":"Ada","firstName":"Eve"`, 1),
			status:   400,
		},
		{name: "trailing input", method: "POST", document: validCreate + `{}`, status: 400},
		{
			name:     "null name",
			method:   "POST",
			document: strings.Replace(validCreate, `"firstName":"Ada"`, `"firstName":null`, 1),
			status:   422,
		},
		{
			name:     "wrong type",
			method:   "POST",
			document: strings.Replace(validCreate, `"firstName":"Ada"`, `"firstName":7`, 1),
			status:   422,
		},
		{
			name:     "false terms",
			method:   "POST",
			document: strings.Replace(validCreate, `"termsAccepted":true`, `"termsAccepted":false`, 1),
			status:   422,
		},
		{
			name:     "missing terms",
			method:   "POST",
			document: strings.Replace(validCreate, `,"termsAccepted":true`, "", 1),
			status:   422,
		},
		{
			name:     "surrounding name whitespace",
			method:   "POST",
			document: strings.Replace(validCreate, `"firstName":"Ada"`, `"firstName":" Ada"`, 1),
			status:   422,
		},
		{
			name:     "control in name",
			method:   "POST",
			document: strings.Replace(validCreate, `"firstName":"Ada"`, `"firstName":"A\u0000da"`, 1),
			status:   422,
		},
		{
			name:     "invalid email",
			method:   "POST",
			document: strings.Replace(validCreate, ` Ada@EXAMPLE.COM \t`, `invalid`, 1),
			status:   422,
		},
		{
			name:     "invalid phone",
			method:   "POST",
			document: strings.Replace(validCreate, `\n+358401234567 `, `123`, 1),
			status:   422,
		},
		{name: "empty patch", method: "PATCH", document: `{}`, status: 422},
		{name: "immutable patch id", method: "PATCH", document: `{"id":"other"}`, status: 422},
		{name: "immutable patch terms", method: "PATCH", document: `{"termsAccepted":true}`, status: 422},
		{name: "null patch", method: "PATCH", document: `{"firstName":null}`, status: 422},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryProfileStore()
			router := newProfileTestRouter(t, store, &profileVerifier{})
			response := profileRequest(t, router, test.method, strings.NewReader(test.document), "token-a")
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
			wantCode := portable.CodeValidationFailed
			if test.status == 400 {
				wantCode = portable.CodeInvalidRequest
			}
			if problem := decodeProblem(t, response); problem.Code != wantCode {
				t.Fatalf("problem=%#v", problem)
			}
			if calls, writes := store.counts(); calls != 0 || writes != 0 {
				t.Fatalf("store calls=%d writes=%d", calls, writes)
			}
		})
	}
}

func TestProfileRequestOrderingAndAuthenticationFailures(t *testing.T) {
	t.Run("semantic validation follows authentication", func(t *testing.T) {
		store := newMemoryProfileStore()
		verifier := &profileVerifier{err: auth.ErrTokenExpired}
		router := newProfileTestRouter(t, store, verifier)
		response := profileRequest(t, router, http.MethodPost, strings.NewReader(`{"bad":`), "token-a")
		if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" ||
			decodeProblem(t, response).Code != portable.CodeUnauthorized {
			t.Fatalf("status=%d headers=%#v body=%s", response.Code, response.Header(), response.Body.String())
		}
		if calls, _ := store.counts(); calls != 0 || verifier.callCount() != 1 {
			t.Fatalf("store calls=%d verifier calls=%d", calls, verifier.callCount())
		}
	})

	t.Run("declared size rejects before authentication", func(t *testing.T) {
		store := newMemoryProfileStore()
		verifier := &profileVerifier{}
		router := newProfileTestRouter(t, store, verifier)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", strings.NewReader("{}"))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Length", "1000001")
		request.ContentLength = 1_000_001
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusRequestEntityTooLarge ||
			decodeProblem(t, response).Code != portable.CodePayloadTooLarge {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if verifier.callCount() != 0 {
			t.Fatalf("verifier called %d times", verifier.callCount())
		}
	})

	t.Run("unsupported media rejects before authentication", func(t *testing.T) {
		verifier := &profileVerifier{}
		router := newProfileTestRouter(t, newMemoryProfileStore(), verifier)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/profile",
			strings.NewReader(validCreate),
		)
		request.Header.Set("Content-Type", "text/plain")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnsupportedMediaType || verifier.callCount() != 0 {
			t.Fatalf("status=%d verifier calls=%d body=%s", response.Code, verifier.callCount(), response.Body.String())
		}
	})

	t.Run("streamed size rejects after authentication", func(t *testing.T) {
		store := newMemoryProfileStore()
		verifier := &profileVerifier{}
		router := newProfileTestRouter(t, store, verifier)
		body := io.NopCloser(strings.NewReader(strings.Repeat(" ", int(portable.MaxRequestBodyBytes+1))))
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", body)
		request.ContentLength = -1
		request.TransferEncoding = []string{"chunked"}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer token-a")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusRequestEntityTooLarge || verifier.callCount() != 1 {
			t.Fatalf("status=%d verifier calls=%d body=%s", response.Code, verifier.callCount(), response.Body.String())
		}
		if calls, _ := store.counts(); calls != 0 {
			t.Fatalf("store calls=%d", calls)
		}
	})
}

func TestProfileExactRequestBodyLimit(t *testing.T) {
	store := newMemoryProfileStore()
	router := newProfileTestRouter(t, store, &profileVerifier{})
	padding := strings.Repeat(" ", int(portable.MaxRequestBodyBytes)-len(validCreate))
	document := validCreate + padding
	if len(document) != int(portable.MaxRequestBodyBytes) {
		t.Fatalf("fixture length=%d", len(document))
	}
	response := profileRequest(t, router, http.MethodPost, strings.NewReader(document), "token-a")
	if response.Code != http.StatusCreated {
		t.Fatalf("exact limit status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProfileMapsPersistenceOutcomesSafely(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
		code portable.Code
	}{
		{name: "not found", err: profilesvc.ErrNotFound, want: 404, code: portable.CodeProfileNotFound},
		{name: "already exists", err: profilesvc.ErrAlreadyExists, want: 409, code: portable.CodeProfileExists},
		{name: "unavailable", err: profilesvc.ErrUnavailable, want: 503, code: portable.CodeDependencyUnavailable},
		{name: "deadline", err: context.DeadlineExceeded, want: 503, code: portable.CodeDependencyUnavailable},
		{
			name: "unexpected",
			err:  errors.New("secret database diagnostic"),
			want: 500,
			code: portable.CodeInternalError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryProfileStore()
			store.err = test.err
			router := newProfileTestRouter(t, store, &profileVerifier{})
			method, body := http.MethodGet, io.Reader(nil)
			if errors.Is(test.err, profilesvc.ErrAlreadyExists) {
				method, body = http.MethodPost, strings.NewReader(validCreate)
			}
			response := profileRequest(t, router, method, body, "token-a")
			if response.Code != test.want || decodeProblem(t, response).Code != test.code ||
				strings.Contains(response.Body.String(), "secret") || response.Header().Get("Retry-After") != "" {
				t.Fatalf("status=%d headers=%#v body=%s", response.Code, response.Header(), response.Body.String())
			}
		})
	}
}

func TestProfileConcurrentCreateAndDeleteAreAtomic(t *testing.T) {
	store := newMemoryProfileStore()
	router := newProfileTestRouter(t, store, &profileVerifier{})
	runConcurrent := func(method string, body func() io.Reader) []int {
		t.Helper()
		start := make(chan struct{})
		results := make(chan int, 2)
		var wait sync.WaitGroup
		for range 2 {
			wait.Go(func() {
				<-start
				results <- profileRequest(t, router, method, body(), "token-a").Code
			})
		}
		close(start)
		wait.Wait()
		close(results)
		statuses := make([]int, 0, 2)
		for status := range results {
			statuses = append(statuses, status)
		}
		return statuses
	}
	statuses := runConcurrent(http.MethodPost, func() io.Reader { return strings.NewReader(validCreate) })
	if !containsStatusPair(statuses, http.StatusCreated, http.StatusConflict) {
		t.Fatalf("create statuses=%v", statuses)
	}
	if _, writes := store.counts(); writes != 1 {
		t.Fatalf("create writes=%d", writes)
	}
	statuses = runConcurrent(http.MethodDelete, func() io.Reader { return nil })
	if !containsStatusPair(statuses, http.StatusNoContent, http.StatusNotFound) {
		t.Fatalf("delete statuses=%v", statuses)
	}
	if _, writes := store.counts(); writes != 2 {
		t.Fatalf("total writes=%d", writes)
	}
}

func TestProfileConcurrentPatchesAndPatchDeleteRemainAtomic(t *testing.T) {
	t.Run("overlapping patches never expose a mixed state", func(t *testing.T) {
		store := newMemoryProfileStore()
		router := newProfileTestRouter(t, store, &profileVerifier{})
		if response := profileRequest(
			t,
			router,
			http.MethodPost,
			strings.NewReader(validCreate),
			"token-a",
		); response.Code != http.StatusCreated {
			t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
		}
		type patchResult struct {
			index    int
			response *httptest.ResponseRecorder
		}
		bodies := []string{
			`{"firstName":"Alpha","lastName":"One"}`,
			`{"firstName":"Beta","lastName":"Two"}`,
		}
		start := make(chan struct{})
		results := make(chan patchResult, len(bodies))
		var wait sync.WaitGroup
		for index, body := range bodies {
			wait.Go(func() {
				<-start
				results <- patchResult{
					index: index,
					response: profileRequest(
						t, router, http.MethodPatch, strings.NewReader(body), "token-a",
					),
				}
			})
		}
		close(start)
		wait.Wait()
		close(results)
		for result := range results {
			if result.response.Code != http.StatusOK {
				t.Fatalf(
					"patch %d status=%d body=%s",
					result.index,
					result.response.Code,
					result.response.Body.String(),
				)
			}
			profile := decodeProfile(t, result.response)
			wantFirst, wantLast := "Alpha", "One"
			if result.index == 1 {
				wantFirst, wantLast = "Beta", "Two"
			}
			if profile.FirstName != wantFirst || profile.LastName != wantLast {
				t.Fatalf("patch %d returned mixed state %#v", result.index, profile)
			}
		}
		stored, err := store.Get(t.Context(), "principal-a")
		if err != nil || stored.FirstName == "Alpha" && stored.LastName != "One" ||
			stored.FirstName == "Beta" && stored.LastName != "Two" ||
			stored.FirstName != "Alpha" && stored.FirstName != "Beta" {
			t.Fatalf("stored profile=%#v err=%v", stored, err)
		}
		if _, writes := store.counts(); writes != 3 {
			t.Fatalf("committed writes=%d want=3", writes)
		}
	})

	t.Run("patch racing with delete never resurrects", func(t *testing.T) {
		store := newMemoryProfileStore()
		router := newProfileTestRouter(t, store, &profileVerifier{})
		if response := profileRequest(
			t,
			router,
			http.MethodPost,
			strings.NewReader(validCreate),
			"token-a",
		); response.Code != http.StatusCreated {
			t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
		}
		start := make(chan struct{})
		patchStatus := make(chan int, 1)
		deleteStatus := make(chan int, 1)
		var wait sync.WaitGroup
		wait.Go(func() {
			<-start
			patchStatus <- profileRequest(
				t, router, http.MethodPatch, strings.NewReader(`{"firstName":"Grace"}`), "token-a",
			).Code
		})
		wait.Go(func() {
			<-start
			deleteStatus <- profileRequest(t, router, http.MethodDelete, nil, "token-a").Code
		})
		close(start)
		wait.Wait()
		patch, deleted := <-patchStatus, <-deleteStatus
		if deleted != http.StatusNoContent || patch != http.StatusOK && patch != http.StatusNotFound {
			t.Fatalf("patch status=%d delete status=%d", patch, deleted)
		}
		get := profileRequest(t, router, http.MethodGet, nil, "token-a")
		if get.Code != http.StatusNotFound || decodeProblem(t, get).Code != portable.CodeProfileNotFound {
			t.Fatalf("post-race get status=%d body=%s", get.Code, get.Body.String())
		}
		_, writes := store.counts()
		wantWrites := 2
		if patch == http.StatusOK {
			wantWrites = 3
		}
		if writes != wantWrites {
			t.Fatalf("committed writes=%d want=%d", writes, wantWrites)
		}
	})
}

func TestProfileUnconfirmedMutationReturnsFailureAndCanBeReconciled(t *testing.T) {
	store := &uncertainProfileStore{memoryProfileStore: newMemoryProfileStore()}
	router := newProfileTestRouter(t, store, &profileVerifier{})
	if response := profileRequest(
		t,
		router,
		http.MethodPost,
		strings.NewReader(validCreate),
		"token-a",
	); response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	update := profileRequest(
		t, router, http.MethodPatch, strings.NewReader(`{"firstName":"Reconciled"}`), "token-a",
	)
	if update.Code != http.StatusServiceUnavailable ||
		decodeProblem(t, update).Code != portable.CodeDependencyUnavailable {
		t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
	}
	get := profileRequest(t, router, http.MethodGet, nil, "token-a")
	profile := decodeProfile(t, get)
	if get.Code != http.StatusOK || profile.FirstName != "Reconciled" {
		t.Fatalf("reconciled get status=%d profile=%#v body=%s", get.Code, profile, get.Body.String())
	}
	if _, writes := store.counts(); writes != 2 {
		t.Fatalf("committed writes=%d want=2", writes)
	}
}

func containsStatusPair(statuses []int, left, right int) bool {
	return len(statuses) == 2 &&
		(statuses[0] == left && statuses[1] == right || statuses[0] == right && statuses[1] == left)
}

func TestProfileCBORRequestAndResponse(t *testing.T) {
	store := newMemoryProfileStore()
	router := newProfileTestRouter(t, store, &profileVerifier{})
	document, err := cbor.Marshal(map[string]any{
		"firstName": "Ada", "lastName": "Lovelace", "contactEmail": "Ada@EXAMPLE.COM",
		"phoneNumber": "+358401234567", "termsAccepted": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", bytes.NewReader(document))
	request.Header.Set("Content-Type", "application/cbor")
	request.Header.Set("Accept", "application/cbor")
	request.Header.Set("Authorization", "Bearer token-a")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Content-Type") != "application/cbor" {
		t.Fatalf(
			"status=%d content-type=%q body=%x",
			response.Code,
			response.Header().Get("Content-Type"),
			response.Body.Bytes(),
		)
	}
	var profile Profile
	if err := cbor.Unmarshal(response.Body.Bytes(), &profile); err != nil || profile.ID != "principal-a" ||
		profile.ContactEmail != "Ada@example.com" {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
}
