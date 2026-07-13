package profile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/janisto/huma-playground/internal/platform/auth"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type stubVerifier struct {
	User  *auth.FirebaseUser
	Error error
}

func (v *stubVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	return v.User, v.Error
}

func testUser() *auth.FirebaseUser {
	return &auth.FirebaseUser{UID: "test-user-123", Email: "test@example.com", EmailVerified: true}
}

type mockService struct {
	profile *profilesvc.Profile
	err     error
}

type memoryStore struct {
	mu       sync.Mutex
	profiles map[string]*profilesvc.Profile
}

func newMemoryStore() *memoryStore {
	return &memoryStore{profiles: make(map[string]*profilesvc.Profile)}
}

func (s *memoryStore) Create(
	_ context.Context,
	userID string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[userID]; exists {
		return nil, profilesvc.ErrAlreadyExists
	}
	now := time.Now().UTC()
	profile := &profilesvc.Profile{
		ID:           userID,
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		ContactEmail: params.ContactEmail,
		PhoneNumber:  params.PhoneNumber,
		Marketing:    params.Marketing,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.profiles[userID] = profile
	return profile, nil
}

func (s *memoryStore) Get(_ context.Context, userID string) (*profilesvc.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, exists := s.profiles[userID]
	if !exists {
		return nil, profilesvc.ErrNotFound
	}
	result := *profile
	return &result, nil
}

func (s *memoryStore) Update(
	context.Context,
	string,
	profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	return nil, errors.New("update is not implemented by this test store")
}

func (s *memoryStore) Delete(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[userID]; !exists {
		return profilesvc.ErrNotFound
	}
	delete(s.profiles, userID)
	return nil
}

func (m *mockService) Create(
	_ context.Context,
	userID string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	if m.err != nil {
		return nil, m.err
	}
	now := time.Now().UTC()
	return &profilesvc.Profile{
		ID:           userID,
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		ContactEmail: params.ContactEmail,
		PhoneNumber:  params.PhoneNumber,
		Marketing:    params.Marketing,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (m *mockService) Get(_ context.Context, _ string) (*profilesvc.Profile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.profile, nil
}

func (m *mockService) Update(_ context.Context, _ string, params profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	if m.err != nil {
		return nil, m.err
	}
	p := *m.profile
	if params.FirstName != nil {
		p.FirstName = *params.FirstName
	}
	if params.LastName != nil {
		p.LastName = *params.LastName
	}
	if params.ContactEmail != nil {
		p.ContactEmail = *params.ContactEmail
	}
	if params.PhoneNumber != nil {
		p.PhoneNumber = *params.PhoneNumber
	}
	if params.Marketing != nil {
		p.Marketing = *params.Marketing
	}
	p.UpdatedAt = time.Now().UTC()
	return &p, nil
}

func (m *mockService) Delete(_ context.Context, _ string) error {
	return m.err
}

func newTestRouter(svc profilesvc.Store, verifier auth.Verifier) chi.Router {
	return newTestRouterWithLogger(svc, verifier, zap.NewNop())
}

func newTestRouterWithLogger(svc profilesvc.Store, verifier auth.Verifier, logger *zap.Logger) chi.Router {
	router := chi.NewRouter()
	router.Use(
		chimiddleware.ClientIPFromRemoteAddr,
	)
	api := humachi.New(router, huma.DefaultConfig("ProfileTest", "test"))
	api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{Logger: logger}))
	api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{Logger: logger}))
	api.UseMiddleware(auth.NewAuthMiddleware(api, verifier))
	Register(api, "/v1", svc)
	return router
}

func testProfile() *profilesvc.Profile {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return &profilesvc.Profile{
		ID:           "test-user-123",
		FirstName:    "John",
		LastName:     "Doe",
		ContactEmail: "john@example.com",
		PhoneNumber:  "+358401234567",
		Marketing:    true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestCreateProfileSuccess(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567","marketing":true}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set(chimiddleware.RequestIDHeader, "create-profile-test")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	location := resp.Header().Get("Location")
	if location != "/v1/profile" {
		t.Errorf("expected Location /v1/profile, got %s", location)
	}

	var profile Profile
	if err := json.Unmarshal(resp.Body.Bytes(), &profile); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if profile.FirstName != "John" {
		t.Errorf("expected firstName John, got %s", profile.FirstName)
	}
	if profile.ContactEmail != "john@example.com" {
		t.Errorf("expected contactEmail john@example.com, got %s", profile.ContactEmail)
	}
}

func TestCreateProfileRejectsLegacyTermsField(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567","marketing":true,"terms":false}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
}

func TestCreateProfileConflict(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrAlreadyExists}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567","marketing":false}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestCreateProfileUnauthorized(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}

	wwwAuth := resp.Header().Get("WWW-Authenticate")
	if wwwAuth != "Bearer" {
		t.Errorf("expected WWW-Authenticate: Bearer, got %s", wwwAuth)
	}
}

func TestGetProfileSuccess(t *testing.T) {
	svc := &mockService{profile: testProfile()}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set(chimiddleware.RequestIDHeader, "get-profile-test")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var profile Profile
	if err := json.Unmarshal(resp.Body.Bytes(), &profile); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if profile.ID != "test-user-123" {
		t.Errorf("expected id test-user-123, got %s", profile.ID)
	}
	if profile.FirstName != "John" {
		t.Errorf("expected firstName John, got %s", profile.FirstName)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrNotFound}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateProfileSuccess(t *testing.T) {
	svc := &mockService{profile: testProfile()}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"Jane"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set(chimiddleware.RequestIDHeader, "update-profile-test")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var profile Profile
	if err := json.Unmarshal(resp.Body.Bytes(), &profile); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if profile.FirstName != "Jane" {
		t.Errorf("expected firstName Jane, got %s", profile.FirstName)
	}
	if profile.LastName != "Doe" {
		t.Errorf("expected lastName Doe (unchanged), got %s", profile.LastName)
	}
}

func TestUpdateProfileEmptyBody(t *testing.T) {
	svc := &mockService{profile: testProfile()}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/profile", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
}

func TestUpdateProfileNotFound(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrNotFound}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"Jane"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileSuccess(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set(chimiddleware.RequestIDHeader, "delete-profile-test")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileNotFound(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrNotFound}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestProfileValidationInvalidEmail(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"invalid-contactEmail","phoneNumber":"+358401234567"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestProfileValidationInvalidPhone(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"12345"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestProfileValidationRejectsSurroundingWhitespace(t *testing.T) {
	router := newTestRouter(&mockService{}, &stubVerifier{User: testUser()})
	body := `{"firstName":" John ","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567"}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", response.Code, response.Body.String())
	}
}

func TestProfileValidationMissingRequired(t *testing.T) {
	svc := &mockService{}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileTwice(t *testing.T) {
	mockSvc := newMemoryStore()
	_, _ = mockSvc.Create(t.Context(), "user-123", profilesvc.CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	})

	verifier := &stubVerifier{
		User: &auth.FirebaseUser{UID: "user-123", Email: "test@example.com"},
	}
	router := newTestRouter(mockSvc, verifier)

	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req1.Header.Set("Authorization", "Bearer valid-token")
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusNoContent {
		t.Fatalf("first delete: expected 204, got %d", resp1.Code)
	}

	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req2.Header.Set("Authorization", "Bearer valid-token")
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusNotFound {
		t.Fatalf("second delete: expected 404, got %d", resp2.Code)
	}
}

func TestCreateProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567","marketing":false}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", problem.Status)
	}
}

func TestProfileUnexpectedErrorIsLoggedOnce(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	router := newTestRouterWithLogger(
		&mockService{err: errors.New("unexpected database error")},
		&stubVerifier{User: testUser()},
		zap.New(core),
	)

	body := `{"firstName":"John","lastName":"Doe","contactEmail":"john@example.com","phoneNumber":"+358401234567","marketing":false}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
	entries := logs.FilterMessage("profile operation failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one profile failure log, got %d", len(entries))
	}
	if operation := entries[0].ContextMap()["operation"]; operation != "create" {
		t.Fatalf("unexpected operation field %#v", operation)
	}
}

func TestProfileUnavailableErrorIsLoggedOnce(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	router := newTestRouterWithLogger(
		&mockService{err: profilesvc.ErrUnavailable},
		&stubVerifier{User: testUser()},
		zap.New(core),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", resp.Code, resp.Body.String())
	}
	entries := logs.FilterMessage("profile store unavailable").All()
	if len(entries) != 1 {
		t.Fatalf("expected one profile unavailable log, got %d", len(entries))
	}
	if operation := entries[0].ContextMap()["operation"]; operation != "get" {
		t.Fatalf("unexpected operation field %#v", operation)
	}
}

func TestGetProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstName":"Jane"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &stubVerifier{User: testUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileConcurrent(t *testing.T) {
	mockSvc := newMemoryStore()
	_, _ = mockSvc.Create(t.Context(), "user-123", profilesvc.CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	})

	verifier := &stubVerifier{
		User: &auth.FirebaseUser{UID: "user-123", Email: "test@example.com"},
	}
	router := newTestRouter(mockSvc, verifier)

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			results <- resp.Code
		})
	}
	wg.Wait()
	close(results)

	var success, notFound int
	for code := range results {
		switch code {
		case http.StatusNoContent:
			success++
		case http.StatusNotFound:
			notFound++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if success != 1 {
		t.Errorf("expected exactly 1 success, got %d", success)
	}
	if notFound != numGoroutines-1 {
		t.Errorf("expected %d not found, got %d", numGoroutines-1, notFound)
	}
}
