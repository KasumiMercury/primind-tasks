package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/worker"
)

func main() {
	cfg := config.Load()

	if cfg.TargetEndpoint == "" {
		log.Fatal("TARGET_ENDPOINT environment variable is required")
	}

	server := worker.NewServer(cfg)

	log.Printf("Starting worker")
	log.Printf("Target endpoint: %s", cfg.TargetEndpoint)
	log.Printf("Queue: %s, Concurrency: %d, Retry: %d", cfg.QueueName, cfg.WorkerConcurrency, cfg.RetryCount)
	log.Printf("Redis: %s", cfg.RedisAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Println("Shutting down worker...")
		server.Shutdown()
	}()

	if err := server.Run(); err != nil {
		log.Fatalf("Failed to run worker: %v", err)
	}
}
