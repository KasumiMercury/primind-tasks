package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/observability/logging"
	obsmw "github.com/KasumiMercury/primind-tasks/internal/observability/middleware"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

type Server struct {
	handler    *Handler
	port       int
	version    string
	httpServer *http.Server
}

func NewServer(cfg *config.Config, client *queue.Client, version string) *Server {
	return &Server{
		handler: NewHandler(client),
		port:    cfg.APIPort,
		version: version,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Health check endpoints
	r.Get("/health/live", s.handler.HealthLive)
	r.Get("/health/ready", s.handler.HealthReady(s.version))
	r.Get("/health", s.handler.HealthReady(s.version))

	// Task creation
	r.Post("/tasks", s.handler.CreateTask)
	r.Post("/tasks/{queue}", s.handler.CreateTaskWithQueue)

	// Task deletion
	r.Delete("/tasks/{taskId}", s.handler.DeleteTask)
	r.Delete("/tasks/{queue}/{taskId}", s.handler.DeleteTaskWithQueue)

	// Wrap with observability middleware
	handler := obsmw.HTTP(r, obsmw.HTTPConfig{
		SkipPaths:  []string{"/health", "/health/live", "/health/ready"},
		Module:     logging.Module("taskqueue"),
		TracerName: "github.com/KasumiMercury/primind-tasks/internal/observability/middleware",
		SpanNameResolver: func(req *http.Request) string {
			routeCtx := chi.RouteContext(req.Context())
			if routeCtx == nil {
				return ""
			}

			pattern := routeCtx.RoutePattern()
			if pattern == "" {
				return ""
			}

			return fmt.Sprintf("%s %s", req.Method, pattern)
		},
	})
	handler = obsmw.PanicRecoveryHTTP(handler)

	return handler
}

func (s *Server) ListenAndServe() error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.Router(),
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}
