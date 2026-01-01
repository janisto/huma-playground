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
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/common"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
	"github.com/janisto/huma-playground/internal/respond"
	"github.com/janisto/huma-playground/internal/routes"
)

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	defer func() {
		if err := common.Sync(); err != nil {
			appmiddleware.LogError(context.Background(), "logger sync error", err)
		}
	}()
	if err := common.Err(); err != nil {
		appmiddleware.LogError(context.Background(), "logger init error", err)
	}
	router := chi.NewRouter()
	router.NotFound(respond.NotFoundHandler())
	router.MethodNotAllowed(respond.MethodNotAllowedHandler())

	// Base middleware stack
	router.Use(
		appmiddleware.Security("/api-docs"),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		appmiddleware.RequestID(),
		// RealIP extracts client IP from X-Real-IP or X-Forwarded-For headers.
		// SECURITY: Only use behind a trusted reverse proxy (e.g., Cloud Run, nginx).
		// Without a trusted proxy, clients can spoof their IP address.
		chimiddleware.RealIP,
		// RequestSize limits request body size to prevent memory exhaustion from large payloads.
		chimiddleware.RequestSize(1<<20), // 1 MB limit
		appmiddleware.RequestLogger(),
		appmiddleware.AccessLogger(),
		respond.Recoverer(),
	)

	cfg := huma.DefaultConfig("Huma Playground API", Version)
	cfg.DocsPath = "/api-docs"
	// Allow JSON fallback for wildcard Accept headers (e.g., */*) since Huma's
	// negotiation uses exact matching and doesn't interpret wildcards per
	// RFC 9110 section 12.5.1. Clients sending unsupported types like text/plain
	// will still receive JSON rather than 406, which is acceptable per RFC 9110
	// section 12.4.1 (servers MAY disregard Accept and return a default).
	api := humachi.New(router, cfg)

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

	// Register routes
	routes.Register(api)

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
		appmiddleware.LogInfo(context.Background(), "server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-listenErr:
		appmiddleware.LogError(context.Background(), "listen failed", err, zap.String("addr", srv.Addr))
		os.Exit(1)
	case <-stop:
		appmiddleware.LogInfo(context.Background(), "shutdown signal received")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		appmiddleware.LogError(ctx, "server shutdown error", err)
	}
	appmiddleware.LogInfo(context.Background(), "server exited")
}
