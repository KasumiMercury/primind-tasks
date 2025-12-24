package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

type Server struct {
	handler *Handler
	port    int
}

func NewServer(cfg *config.Config, client *queue.Client) *Server {
	return &Server{
		handler: NewHandler(client),
		port:    cfg.APIPort,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", s.handler.HealthCheck)

	// Task creation
	r.Post("/tasks", s.handler.CreateTask)
	r.Post("/tasks/{queue}", s.handler.CreateTaskWithQueue)

	// Task deletion
	r.Delete("/tasks/{taskId}", s.handler.DeleteTask)
	r.Delete("/tasks/{queue}/{taskId}", s.handler.DeleteTaskWithQueue)

	return r
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, s.Router())
}
