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
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/janisto/huma-playground/internal/common"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
	"github.com/janisto/huma-playground/internal/respond"
	"github.com/janisto/huma-playground/internal/routes"
	"go.uber.org/zap"
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
	// Ensure responses share envelope + logging behavior.
	respond.Install()
	// Use CLI helper to get startup/shutdown handling & future flags.
	router := chi.NewRouter()
	router.NotFound(respond.NotFoundHandler())
	router.MethodNotAllowed(respond.MethodNotAllowedHandler())

	// Base middleware stack
	router.Use(
		appmiddleware.CORS(),
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		appmiddleware.RequestLogger(),
		appmiddleware.AccessLogger(),
		respond.Recoverer(),
	)

	cfg := huma.DefaultConfig("Huma Playground API", Version)
	/*
		cfg.CreateHooks = append(cfg.CreateHooks, func(c huma.Config) huma.Config {
			c.Transformers = nil // remove the schema link transformer
			return c
		})
	*/
	api := humachi.New(router, cfg)

	// Register routes
	routes.Register(api)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		appmiddleware.LogInfo(context.Background(), "server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appmiddleware.LogFatal(context.Background(), "listen failed", err, zap.String("addr", srv.Addr))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	appmiddleware.LogInfo(context.Background(), "shutdown signal received")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		appmiddleware.LogError(ctx, "server shutdown error", err)
	}
	appmiddleware.LogInfo(context.Background(), "server exited")

}
