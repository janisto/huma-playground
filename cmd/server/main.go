package main

import (
	"context"
	"errors"
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
	"github.com/janisto/huma-observability"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/http/health"
	"github.com/janisto/huma-playground/internal/http/v1/routes"
	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/firebase"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	logger, err := obs.NewLogger(obs.LoggerConfig{
		Preset:    obs.PresetGCP,
		AddCaller: true,
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil && !errors.Is(syncErr, syscall.ENOTTY) {
			logger.Error("logger sync error", zap.Error(syncErr))
		}
	}()

	// Initialize Firebase
	ctx := context.Background()
	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	if firebaseProjectID == "" {
		if os.Getenv("APP_ENVIRONMENT") == "development" {
			firebaseProjectID = "demo-test-project"
			logger.Warn("using demo-test-project for local development")
		} else {
			logger.Fatal("FIREBASE_PROJECT_ID environment variable is required")
		}
	}
	firebaseClients, err := firebase.InitializeClients(ctx, firebase.Config{
		ProjectID: firebaseProjectID,
	})
	if err != nil {
		logger.Fatal("firebase init failed", zap.Error(err))
	}
	defer func() {
		if closeErr := firebaseClients.Close(); closeErr != nil {
			logger.Error("firebase close error", zap.Error(closeErr))
		}
	}()

	// Create auth verifier and profile service
	verifier := auth.NewFirebaseVerifier(firebaseClients.Auth)
	profileService := profilesvc.NewFirestoreStore(firebaseClients.Firestore)

	// Create GitHub service
	githubHTTPClient := &http.Client{Timeout: 10 * time.Second}
	var githubOpts []githubsvc.Option
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		githubOpts = append(githubOpts, githubsvc.WithToken(token))
	}
	githubService := githubsvc.NewClient(githubHTTPClient, githubOpts...)

	router := chi.NewRouter()
	httpAccessLogger := appmiddleware.AccessLogger()
	router.NotFound(httpAccessLogger(respond.NotFoundHandler()).ServeHTTP)
	router.MethodNotAllowed(httpAccessLogger(respond.MethodNotAllowedHandler()).ServeHTTP)

	// Base middleware stack
	router.Use(
		obs.HTTPRequestContext(obs.HTTPRequestContextConfig{
			Logger: logger,
			Preset: obs.PresetGCP,
		}),
		respond.Recoverer(logger),
		appmiddleware.Security("/v1/api-docs"),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		chimiddleware.ClientIPFromRemoteAddr,
		chimiddleware.RequestSize(1<<20), // 1 MB limit
	)

	// Root-level endpoints (unversioned)
	router.Group(func(r chi.Router) {
		r.Use(httpAccessLogger)
		r.Get("/health", health.Handler)
	})

	// Versioned API
	cfg := huma.DefaultConfig("Huma Playground API", Version)
	cfg.DocsPath = "/api-docs"
	cfg.OpenAPIPath = "/openapi"
	cfg.Servers = []*huma.Server{
		{URL: "/v1"},
	}

	router.Route("/v1", func(r chi.Router) {
		api := humachi.New(r, cfg)
		api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{
			Logger: logger,
			Preset: obs.PresetGCP,
		}))
		api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{
			Logger: logger,
			Preset: obs.PresetGCP,
		}))

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

		auth.RegisterSecurityScheme(api)

		routes.Register(api, verifier, profileService, githubService)
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
		logger.Info("server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-listenErr:
		logger.Error("listen failed", zap.Error(err), zap.String("addr", srv.Addr))
		os.Exit(1)
	case <-shutdownCtx.Done():
		logger.Info("shutdown signal received", zap.Error(context.Cause(shutdownCtx)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("server exited")
}
