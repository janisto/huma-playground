package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability"
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

type dependencies struct {
	verifier auth.Verifier
	profiles profilesvc.Store
	github   githubsvc.Service
}

type applicationClients struct {
	*firebase.Clients
	dependencies
}

func newApplicationClients(ctx context.Context, cfg config, logger *zap.Logger) (*applicationClients, error) {
	githubHTTPClient := &http.Client{Timeout: 10 * time.Second}
	var githubOptions []githubsvc.Option
	if cfg.GitHubToken != "" {
		githubOptions = append(githubOptions, githubsvc.WithToken(cfg.GitHubToken))
	}
	githubClient, err := githubsvc.NewClient(githubHTTPClient, githubOptions...)
	if err != nil {
		return nil, fmt.Errorf("create GitHub client: %w", err)
	}

	if cfg.FirebaseMode == firebaseModeOffline {
		logger.Warn("Firebase is offline; protected routes return service unavailable")
		return &applicationClients{dependencies: dependencies{
			verifier: unavailableVerifier{},
			profiles: unavailableProfileStore{},
			github:   githubClient,
		}}, nil
	}
	if cfg.FirebaseMode == firebaseModeEmulator {
		logger.Info("using Firebase emulators", zap.String("project_id", cfg.FirebaseProjectID))
	}
	clients, err := firebase.InitializeClients(ctx, firebase.Config{ProjectID: cfg.FirebaseProjectID})
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase clients: %w", err)
	}
	return &applicationClients{
		Clients: clients,
		dependencies: dependencies{
			verifier: auth.NewFirebaseVerifier(clients.Auth),
			profiles: profilesvc.NewFirestoreStore(clients.Firestore),
			github:   githubClient,
		},
	}, nil
}

func (c *applicationClients) Close() error {
	if c == nil || c.Clients == nil {
		return nil
	}
	return c.Clients.Close()
}

type unavailableVerifier struct{}

func (unavailableVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	return nil, auth.ErrAuthUnavailable
}

type unavailableProfileStore struct{}

func (unavailableProfileStore) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Get(context.Context, string) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Delete(context.Context, string) error {
	return profilesvc.ErrUnavailable
}

func newRouter(cfg config, deps dependencies, logger *zap.Logger) http.Handler {
	apiConfig := huma.DefaultConfig("Huma Playground API", Version)
	apiConfig.DocsPath = "/api-docs"
	apiConfig.OpenAPIPath = "/openapi"
	apiConfig.Servers = []*huma.Server{{URL: cfg.APIPrefix}}
	apiConfig.Info.Description = "A high-quality Huma v2 example API with JSON and CBOR support."
	apiConfig.Info.License = &huma.License{Name: "MIT", URL: "https://opensource.org/license/mit"}
	apiConfig.Tags = []*huma.Tag{
		{Name: "Hello", Description: "Minimal Huma operation examples."},
		{Name: "Items", Description: "Static cursor-pagination example."},
		{Name: "Profile", Description: "Firebase-authenticated Firestore profile CRUD."},
		{Name: "GitHub", Description: "Bounded read-only GitHub API proxy examples."},
	}
	apiConfig.RejectUnknownQueryParameters = true

	apiRouter := chi.NewRouter()
	api := humachi.New(apiRouter, apiConfig)
	api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{Logger: logger, Preset: obs.PresetGCP}))
	api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{Logger: logger, Preset: obs.PresetGCP}))
	addCBOROpenAPIContent(api)
	auth.RegisterSecurityScheme(api)
	routes.Register(api, cfg.APIPrefix, deps.verifier, deps.profiles, deps.github)

	router := chi.NewRouter()
	httpAccessLogger := appmiddleware.AccessLogger()
	apiRouter.NotFound(httpAccessLogger(respond.NotFoundHandler(api)).ServeHTTP)
	apiRouter.MethodNotAllowed(httpAccessLogger(respond.MethodNotAllowedHandler(api)).ServeHTTP)
	router.NotFound(httpAccessLogger(respond.NotFoundHandler(api)).ServeHTTP)
	router.MethodNotAllowed(httpAccessLogger(respond.MethodNotAllowedHandler(api)).ServeHTTP)
	router.Use(
		appmiddleware.IgnoreForwardedHeaders(),
		obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger, Preset: obs.PresetGCP}),
		respond.Recoverer(api, logger),
		requestContextTimeout(cfg.RequestTimeout),
		appmiddleware.Security(cfg.APIPrefix),
		appmiddleware.Vary(),
		appmiddleware.CORS(cfg.CORSOrigins),
		chimiddleware.ClientIPFromRemoteAddr,
		chimiddleware.RequestSize(1<<20),
	)

	router.Group(func(r chi.Router) {
		r.Use(httpAccessLogger)
		r.Get("/health", health.Handler)
	})
	router.Mount(cfg.APIPrefix, apiRouter)
	return router
}

func addCBOROpenAPIContent(api huma.API) {
	api.OpenAPI().OnAddOperation = append(api.OpenAPI().OnAddOperation, func(_ *huma.OpenAPI, op *huma.Operation) {
		if op.RequestBody != nil && op.RequestBody.Content != nil {
			if content, ok := op.RequestBody.Content["application/json"]; ok {
				op.RequestBody.Content["application/cbor"] = content
			}
		}
		for _, response := range op.Responses {
			if response.Content == nil {
				continue
			}
			if content, ok := response.Content["application/json"]; ok {
				response.Content["application/cbor"] = content
			}
			if content, ok := response.Content["application/problem+json"]; ok {
				response.Content["application/problem+cbor"] = content
			}
		}
	})
}

func requestContextTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newServer(cfg config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    64 << 10,
	}
}

func serve(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, logger *zap.Logger) error {
	if ctx.Err() != nil {
		return nil
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	logger.Info("server listening", zap.String("addr", listener.Addr().String()), zap.String("version", Version))

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- server.Serve(listener)
	}()

	select {
	case err := <-listenErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve on %s: %w", server.Addr, err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		closeErr := server.Close()
		return errors.Join(fmt.Errorf("graceful shutdown: %w", err), closeErr)
	}
	if err := <-listenErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server stopped: %w", err)
	}
	return nil
}
