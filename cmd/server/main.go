package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/janisto/huma-observability"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"
)

var Version = "dev"

func main() {
	if err := run(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "huma-playground: %v\n", err)
		os.Exit(1)
	}
}

func run(parent context.Context) (runErr error) {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	logger, err := obs.NewLogger(obs.LoggerConfig{
		Preset:      obs.PresetGCP,
		Level:       cfg.LogLevel,
		AddCaller:   true,
		Development: cfg.Environment == environmentDevelopment,
	})
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil &&
			!errors.Is(syncErr, syscall.ENOTTY) && !errors.Is(syncErr, syscall.EINVAL) {
			runErr = errors.Join(runErr, fmt.Errorf("sync logger: %w", syncErr))
		}
	}()

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	clients, err := newApplicationClients(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := clients.Close(); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Firebase clients: %w", closeErr))
		}
	}()

	server := newServer(cfg, newRouter(cfg, clients.dependencies, logger))
	if err := serve(ctx, server, cfg.ShutdownTimeout, logger); err != nil {
		return err
	}
	if cause := context.Cause(ctx); cause != nil {
		logger.Info("server exited", zap.Error(cause))
	} else {
		logger.Info("server exited")
	}
	return nil
}
