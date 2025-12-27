package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/KasumiMercury/primind-tasks/internal/observability/logging"
	"github.com/KasumiMercury/primind-tasks/internal/observability/tracing"
	"go.opentelemetry.io/otel"
)

type HTTPConfig struct {
	// SkipPaths are paths that skip observability
	SkipPaths []string
	Module    logging.Module
	// ModuleResolver returns a module for the request when module depends on path
	ModuleResolver func(*http.Request) logging.Module
	Worker         bool
	// JobNameResolver returns a job name for worker-style logging
	JobNameResolver func(*http.Request) string
	TracerName      string
	// SpanNameResolver returns a span name for the request
	SpanNameResolver func(*http.Request) string
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	return rw.ResponseWriter.Write(b)
}

func HTTP(next http.Handler, cfg HTTPConfig) http.Handler {
	skipSet := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip observability for configured paths
		if _, skip := skipSet[r.URL.Path]; skip {
			next.ServeHTTP(w, r)

			return
		}

		if isConnectRequest(r) {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now()

		requestID := logging.ValidateAndExtractRequestID(r.Header.Get("x-request-id"))
		ctx := logging.WithRequestID(r.Context(), requestID)
		module := cfg.Module
		if cfg.ModuleResolver != nil {
			module = cfg.ModuleResolver(r)
		}
		if module != "" {
			ctx = logging.WithModule(ctx, module)
		}

		ctx = tracing.ExtractFromHTTPRequest(ctx, r)

		spanName := ""
		if cfg.SpanNameResolver != nil {
			spanName = cfg.SpanNameResolver(r)
		}
		if spanName == "" {
			path := r.Pattern
			if path == "" {
				path = r.URL.Path
			}
			spanName = fmt.Sprintf("%s %s", r.Method, path)
		}

		tracer := otel.Tracer(cfg.TracerName)
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()

		w.Header().Set("x-request-id", requestID)
		r.Header.Set("x-request-id", requestID)

		finishEvent := "http.request.finish"
		finishMessage := "request completed"
		jobName := ""
		if cfg.Worker {
			finishEvent = "job.finish"
			finishMessage = "job finished"
			if cfg.JobNameResolver != nil {
				jobName = cfg.JobNameResolver(r)
			}
			if jobName == "" {
				jobName = r.URL.Path
			}

			startAttrs := []slog.Attr{
				slog.String("event", "job.start"),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("job.name", jobName),
				slog.String("job.id", requestID),
			}
			slog.LogAttrs(ctx, slog.LevelInfo, "job started", startAttrs...)
		}

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start)

		finishAttrs := []slog.Attr{
			slog.String("event", finishEvent),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Int("status", wrapped.status),
			slog.Duration("duration", duration),
		}
		if cfg.Worker {
			finishAttrs = append(finishAttrs,
				slog.String("job.name", jobName),
				slog.String("job.id", requestID),
			)
		}
		slog.LogAttrs(ctx, slog.LevelInfo, finishMessage, finishAttrs...)
	})
}

func isConnectRequest(r *http.Request) bool {
	if r.Header.Get("Connect-Protocol-Version") != "" {
		return true
	}
	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/connect")
}
