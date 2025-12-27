package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/KasumiMercury/primind-tasks/internal/observability/logging"
	"github.com/KasumiMercury/primind-tasks/internal/observability/tracing"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

type HTTPForwardHandler struct {
	targetEndpoint string
	httpClient     *http.Client
}

func NewHTTPForwardHandler(targetEndpoint string, timeout time.Duration) *HTTPForwardHandler {
	return &HTTPForwardHandler{
		targetEndpoint: targetEndpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (h *HTTPForwardHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	taskID := t.ResultWriter().TaskID()
	taskType := t.Type()
	jobName := strings.ReplaceAll(taskType, ":", ".")

	ctx = logging.WithModule(ctx, logging.Module("taskqueue"))
	status := "success"
	httpStatus := 0
	started := false
	logStart := func() {
		if started {
			return
		}
		started = true
		slog.InfoContext(ctx, "job started",
			slog.String("event", "job.start"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
		)
	}
	defer func() {
		if !started {
			logStart()
		}
		attrs := []slog.Attr{
			slog.String("event", "job.finish"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
			slog.String("job.status", status),
		}
		if httpStatus != 0 {
			attrs = append(attrs, slog.Int("http.status_code", httpStatus))
		}
		slog.LogAttrs(ctx, slog.LevelInfo, "job finished", attrs...)
	}()

	payload, err := queue.UnmarshalTaskPayload(t.Payload())
	if err != nil {
		status = "fail"
		logStart()
		slog.ErrorContext(ctx, "job failed",
			slog.String("event", "job.fail"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
			slog.String("error", err.Error()),
			slog.String("reason", "unmarshal_error"),
		)
		return fmt.Errorf("unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	// Extract trace context from task headers (restore as remote parent)
	ctx = tracing.ExtractFromMap(ctx, payload.Headers)

	// Extract x-request-id from task headers
	reqID := logging.ValidateAndExtractRequestID(payload.Headers["x-request-id"])
	ctx = logging.WithRequestID(ctx, reqID)

	messageType := payload.Headers["message_type"]
	if messageType == "" {
		messageType = payload.Headers["event_type"]
	}
	if messageType != "" {
		jobName = messageType
	}

	tracer := otel.Tracer("github.com/KasumiMercury/primind-tasks/internal/worker")
	ctx, span := tracer.Start(ctx, "task.process")
	span.SetAttributes(
		attribute.String("job.name", jobName),
		attribute.String("task.type", taskType),
	)
	defer span.End()

	logStart()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.targetEndpoint, bytes.NewReader(payload.Body))
	if err != nil {
		status = "fail"
		slog.ErrorContext(ctx, "job failed",
			slog.String("event", "job.fail"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
			slog.String("error", err.Error()),
			slog.String("reason", "request_creation_error"),
		)
		return fmt.Errorf("create request: %w: %w", err, asynq.SkipRetry)
	}

	for k, v := range payload.Headers {
		req.Header.Set(k, v)
	}

	// Inject trace context into outgoing request
	tracing.InjectToHTTPRequest(ctx, req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		status = "fail"
		slog.ErrorContext(ctx, "job failed",
			slog.String("event", "job.fail"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
			slog.String("error", err.Error()),
			slog.String("reason", "http_error"),
		)
		return fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "failed to close response body",
				slog.String("job.name", jobName),
				slog.String("job.id", taskID),
				slog.String("error", err.Error()),
			)
		}
	}()
	httpStatus = resp.StatusCode

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		status = "fail"
		slog.ErrorContext(ctx, "job failed",
			slog.String("event", "job.fail"),
			slog.String("job.name", jobName),
			slog.String("job.id", taskID),
			slog.Int("http.status_code", resp.StatusCode),
			slog.String("reason", "client_error"),
			slog.String("response.body", string(body)),
		)
		return fmt.Errorf("client error %d: %w", resp.StatusCode, asynq.SkipRetry)
	}

	status = "fail"
	slog.WarnContext(ctx, "job failed (will retry)",
		slog.String("event", "job.fail"),
		slog.String("job.name", jobName),
		slog.String("job.id", taskID),
		slog.Int("http.status_code", resp.StatusCode),
		slog.String("reason", "server_error"),
		slog.String("response.body", string(body)),
	)
	return fmt.Errorf("server error %d", resp.StatusCode)
}
