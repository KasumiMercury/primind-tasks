package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/worker"
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

	if cfg.TargetEndpoint == "" {
		slog.Error("TARGET_ENDPOINT environment variable is required")

		return errors.New("TARGET_ENDPOINT environment variable is required")
	}

	server := worker.NewServer(cfg)

	slog.InfoContext(ctx, "starting worker",
		slog.String("event", "worker.start"),
		slog.String("target_endpoint", cfg.TargetEndpoint),
		slog.String("queue", cfg.QueueName),
		slog.Int("concurrency", cfg.WorkerConcurrency),
		slog.Int("retry", cfg.RetryCount),
		slog.String("redis", cfg.RedisAddr),
		slog.String("version", Version),
	)

	go func() {
		<-ctx.Done()

		slog.InfoContext(ctx, "shutdown signal received, shutting down worker...",
			slog.String("event", "worker.shutdown.start"),
		)

		server.Shutdown()
	}()

	if err := server.Run(); err != nil {
		slog.ErrorContext(ctx, "worker exited with error",
			slog.String("event", "worker.exit.fail"),
			slog.String("error", err.Error()),
		)

		return err
	}

	slog.InfoContext(ctx, "worker stopped",
		slog.String("event", "worker.stop"),
	)

	return nil
}
