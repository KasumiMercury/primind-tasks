package main

import (
	"log"

	"github.com/KasumiMercury/primind-tasks/internal/api"
	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

func main() {
	cfg := config.Load()

	client := queue.NewClient(cfg)
	defer client.Close()

	server := api.NewServer(cfg, client)

	log.Printf("Starting API server on port %d", cfg.APIPort)
	log.Printf("Queue: %s, Redis: %s", cfg.QueueName, cfg.RedisAddr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
