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

	"github.com/janisto/huma-playground/internal/platform/auth"
	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type mockService struct {
	profile *profilesvc.Profile
	err     error
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
		ID:          userID,
		Firstname:   params.Firstname,
		Lastname:    params.Lastname,
		Email:       params.Email,
		PhoneNumber: params.PhoneNumber,
		Marketing:   params.Marketing,
		Terms:       params.Terms,
		CreatedAt:   now,
		UpdatedAt:   now,
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
	if params.Firstname != nil {
		p.Firstname = *params.Firstname
	}
	if params.Lastname != nil {
		p.Lastname = *params.Lastname
	}
	if params.Email != nil {
		p.Email = *params.Email
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

func newTestRouter(svc profilesvc.Service, verifier auth.Verifier) chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("ProfileTest", "test"))
	api.UseMiddleware(auth.NewAuthMiddleware(api, verifier))
	Register(api, svc)
	return router
}

func testProfile() *profilesvc.Profile {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return &profilesvc.Profile{
		ID:          "test-user-123",
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   true,
		Terms:       true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestCreateProfileSuccess(t *testing.T) {
	svc := &mockService{}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":true,"terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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

	if profile.Firstname != "John" {
		t.Errorf("expected firstname John, got %s", profile.Firstname)
	}
	if profile.Email != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", profile.Email)
	}
}

func TestCreateProfileTermsRequired(t *testing.T) {
	svc := &mockService{}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":true,"terms":false}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":false,"terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
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
	if profile.Firstname != "John" {
		t.Errorf("expected firstname John, got %s", profile.Firstname)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrNotFound}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateProfileSuccess(t *testing.T) {
	svc := &mockService{profile: testProfile()}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"Jane"}`
	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
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

	if profile.Firstname != "Jane" {
		t.Errorf("expected firstname Jane, got %s", profile.Firstname)
	}
	if profile.Lastname != "Doe" {
		t.Errorf("expected lastname Doe (unchanged), got %s", profile.Lastname)
	}
}

func TestUpdateProfileNotFound(t *testing.T) {
	svc := &mockService{err: profilesvc.ErrNotFound}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"Jane"}`
	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodDelete, "/profile", nil)
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestProfileValidationInvalidEmail(t *testing.T) {
	svc := &mockService{}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"invalid-email","phoneNumber":"+358401234567","terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"12345","terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestProfileValidationMissingRequired(t *testing.T) {
	svc := &mockService{}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John"}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileTwice(t *testing.T) {
	mockSvc := profilesvc.NewMockProfileService()
	_, _ = mockSvc.Create(context.Background(), "user-123", profilesvc.CreateParams{
		Firstname: "Test",
		Lastname:  "User",
		Email:     "test@example.com",
		Terms:     true,
	})

	verifier := &auth.MockVerifier{
		User: &auth.FirebaseUser{UID: "user-123", Email: "test@example.com"},
	}
	router := newTestRouter(mockSvc, verifier)

	req1 := httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req1.Header.Set("Authorization", "Bearer valid-token")
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusNoContent {
		t.Fatalf("first delete: expected 204, got %d", resp1.Code)
	}

	req2 := httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req2.Header.Set("Authorization", "Bearer valid-token")
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusNotFound {
		t.Fatalf("second delete: expected 404, got %d", resp2.Code)
	}
}

func TestCreateProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":false,"terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
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

func TestGetProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateProfileInternalServerError(t *testing.T) {
	svc := &mockService{err: errors.New("unexpected database error")}
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	body := `{"firstname":"Jane"}`
	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
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
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	router := newTestRouter(svc, verifier)

	req := httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteProfileConcurrent(t *testing.T) {
	mockSvc := profilesvc.NewMockProfileService()
	_, _ = mockSvc.Create(context.Background(), "user-123", profilesvc.CreateParams{
		Firstname: "Test",
		Lastname:  "User",
		Email:     "test@example.com",
		Terms:     true,
	})

	verifier := &auth.MockVerifier{
		User: &auth.FirebaseUser{UID: "user-123", Email: "test@example.com"},
	}
	router := newTestRouter(mockSvc, verifier)

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodDelete, "/profile", nil)
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
