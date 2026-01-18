package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/http/health"
	"github.com/janisto/huma-playground/internal/http/v1/routes"
	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/firebase"
	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	defer func() {
		if err := applog.Sync(); err != nil {
			applog.LogError(context.Background(), "logger sync error", err)
		}
	}()
	if err := applog.Err(); err != nil {
		applog.LogError(context.Background(), "logger init error", err)
	}

	// Initialize Firebase
	ctx := context.Background()
	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	if firebaseProjectID == "" {
		if os.Getenv("APP_ENVIRONMENT") == "development" {
			firebaseProjectID = "demo-test-project"
			applog.LogWarn(ctx, "using demo-test-project for local development")
		} else {
			applog.LogFatal(ctx, "FIREBASE_PROJECT_ID environment variable is required", nil)
		}
	}
	firebaseClients, err := firebase.InitializeClients(ctx, firebase.Config{
		ProjectID:                    firebaseProjectID,
		GoogleApplicationCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	})
	if err != nil {
		applog.LogFatal(ctx, "firebase init failed", err)
	}
	defer func() {
		if closeErr := firebaseClients.Close(); closeErr != nil {
			applog.LogError(ctx, "firebase close error", closeErr)
		}
	}()

	// Create auth verifier and profile service
	verifier := auth.NewFirebaseVerifier(firebaseClients.Auth)
	profileService := profilesvc.NewFirestoreStore(firebaseClients.Firestore)

	router := chi.NewRouter()
	router.NotFound(respond.NotFoundHandler())
	router.MethodNotAllowed(respond.MethodNotAllowedHandler())

	// Base middleware stack
	router.Use(
		appmiddleware.Security("/v1/api-docs"),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		appmiddleware.RequestID(),
		// RealIP extracts client IP from X-Real-IP or X-Forwarded-For headers.
		// SECURITY: Only use behind a trusted reverse proxy (e.g., Cloud Run, nginx).
		// Without a trusted proxy, clients can spoof their IP address.
		chimiddleware.RealIP,
		// RequestSize limits request body size to prevent memory exhaustion from large payloads.
		chimiddleware.RequestSize(1<<20), // 1 MB limit
		applog.RequestLogger(),
		applog.AccessLogger(),
		respond.Recoverer(),
	)

	// Root-level endpoints (unversioned)
	router.Get("/health", health.Handler)

	// Versioned API
	cfg := huma.DefaultConfig("Huma Playground API", Version)
	cfg.DocsPath = "/api-docs"
	cfg.OpenAPIPath = "/openapi"
	cfg.Servers = []*huma.Server{
		{URL: "/v1"},
	}

	router.Route("/v1", func(r chi.Router) {
		api := humachi.New(r, cfg)

		// Add CBOR content type to OpenAPI requests and responses
		api.OpenAPI().OnAddOperation = append(api.OpenAPI().OnAddOperation,
			func(_ *huma.OpenAPI, op *huma.Operation) {
				if op.RequestBody != nil && op.RequestBody.Content != nil {
					if jsonContent, ok := op.RequestBody.Content["application/json"]; ok {
						op.RequestBody.Content["application/cbor"] = jsonContent
					}
				}
				for _, resp := range op.Responses {
					if resp.Content == nil {
						continue
					}
					if jsonContent, ok := resp.Content["application/json"]; ok {
						resp.Content["application/cbor"] = jsonContent
					}
				}
			},
		)

		// OpenAPI security scheme for Firebase JWT authentication
		api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "Firebase ID token",
			},
		}

		routes.Register(api, verifier, profileService)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10, // 64 KB
	}

	listenErr := make(chan error, 1)
	go func() {
		applog.LogInfo(context.Background(), "server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-listenErr:
		applog.LogError(context.Background(), "listen failed", err, zap.String("addr", srv.Addr))
		os.Exit(1)
	case <-stop:
		applog.LogInfo(context.Background(), "shutdown signal received")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		applog.LogError(ctx, "server shutdown error", err)
	}
	applog.LogInfo(context.Background(), "server exited")
}
