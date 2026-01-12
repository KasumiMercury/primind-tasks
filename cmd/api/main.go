package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KasumiMercury/primind-tasks/internal/api"
	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

// Version is set via ldflags at build time
var Version = "dev"

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	obs, err := initObservability(ctx)
	if err != nil {
		slog.Error("failed to initialize observability", slog.String("error", err.Error()))

		return err
	}

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := obs.Shutdown(shutdownCtx); err != nil {
			slog.Warn("observability shutdown error", slog.String("error", err.Error()))
		}
	}()

	slog.SetDefault(obs.Logger())

	cfg := config.Load()

	client := queue.NewClient(cfg)

	defer func() {
		if err := client.Close(); err != nil {
			slog.Warn("failed to close queue client", slog.String("error", err.Error()))
		}
	}()

	server := api.NewServer(cfg, client, Version)

	slog.InfoContext(ctx, "starting API server",
		slog.String("event", "server.start"),
		slog.Int("port", cfg.APIPort),
		slog.String("queue", cfg.QueueName),
		slog.String("redis", cfg.RedisAddr),
		slog.String("version", Version),
	)

	go func() {
		<-ctx.Done()

		slog.InfoContext(ctx, "shutdown signal received",
			slog.String("event", "server.shutdown.start"),
		)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown server", slog.String("error", err.Error()))
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, "server exited with error",
			slog.String("event", "server.exit.fail"),
			slog.String("error", err.Error()),
		)

		return err
	}

	slog.InfoContext(ctx, "server stopped",
		slog.String("event", "server.stop"),
	)

	return nil
}
