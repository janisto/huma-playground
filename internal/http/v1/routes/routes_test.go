package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/janisto/huma-playground/internal/platform/auth"
	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type mockProfileService struct{}

func (m *mockProfileService) Create(
	_ context.Context,
	userID string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
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

func (m *mockProfileService) Get(_ context.Context, userID string) (*profilesvc.Profile, error) {
	return &profilesvc.Profile{
		ID:        userID,
		Firstname: "Test",
		Lastname:  "User",
		Email:     "test@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockProfileService) Update(
	_ context.Context,
	userID string,
	_ profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	return &profilesvc.Profile{
		ID:        userID,
		Firstname: "Updated",
		Lastname:  "User",
		Email:     "test@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockProfileService) Delete(_ context.Context, _ string) error {
	return nil
}

func newTestRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("RoutesTest", "test"))
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	profileService := &mockProfileService{}
	Register(api, verifier, profileService)
	return router
}

func TestRegisterRoutesHello(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-hello")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRegisterRoutesItems(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-items")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
