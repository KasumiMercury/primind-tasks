package worker

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/hibiken/asynq"

	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

type Server struct {
	server  *asynq.Server
	handler *HTTPForwardHandler
}

func NewServer(cfg *config.Config) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				cfg.QueueName: 1,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				return time.Duration(math.Pow(2, float64(n))) * 10 * time.Second
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				log.Printf("task %s failed (retry %d/%d): %v", task.Type(), retried, maxRetry, err)
			}),
		},
	)

	handler := NewHTTPForwardHandler(cfg.TargetEndpoint, cfg.RequestTimeout)

	return &Server{
		server:  srv,
		handler: handler,
	}
}

func (s *Server) Run() error {
	mux := asynq.NewServeMux()
	mux.Handle(queue.TaskTypeHTTPForward, s.handler)
	return s.server.Run(mux)
}

func (s *Server) Shutdown() {
	s.server.Shutdown()
}
