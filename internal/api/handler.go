package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	taskqueuev1 "github.com/KasumiMercury/primind-tasks/internal/gen/taskqueue/v1"
	"github.com/KasumiMercury/primind-tasks/internal/observability/logging"
	"github.com/KasumiMercury/primind-tasks/internal/observability/tracing"
	pjson "github.com/KasumiMercury/primind-tasks/internal/proto"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
)

type Handler struct {
	client *queue.Client
}

func NewHandler(client *queue.Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("failed to read request body: %v", err))
		return
	}

	var req taskqueuev1.CreateTaskRequest
	if err := pjson.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if err := pjson.Validate(&req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("validation error: %v", err))
		return
	}

	decodedBody, err := base64.StdEncoding.DecodeString(req.Task.HttpRequest.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid base64 body: %v", err))
		return
	}

	payload := queue.NewTaskPayload(decodedBody, req.Task.HttpRequest.Headers)

	// Inject trace context (traceparent/tracestate) into task headers
	tracing.InjectToMap(r.Context(), payload.Headers)

	// Inject x-request-id into task headers
	reqID := logging.RequestIDFromContext(r.Context())
	if reqID == "" {
		reqID = logging.ValidateAndExtractRequestID("")
	}
	payload.Headers["x-request-id"] = reqID

	var scheduleTime *time.Time
	if req.Task.ScheduleTime != "" {
		t, err := time.Parse(time.RFC3339, req.Task.ScheduleTime)
		if err != nil {
			WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid scheduleTime format: %v", err))
			return
		}
		scheduleTime = &t
	}

	info, err := h.client.EnqueueTask(payload, scheduleTime, req.Task.Name)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			WriteError(w, http.StatusConflict, StatusAlreadyExists, fmt.Sprintf("task with name %q already exists", req.Task.Name))
			return
		}
		slog.ErrorContext(r.Context(), "failed to enqueue task",
			slog.String("event", "task.enqueue.fail"),
			slog.String("error", err.Error()),
		)
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to enqueue task")
		return
	}

	taskName := req.Task.Name
	if taskName == "" {
		taskName = fmt.Sprintf("tasks/%s", info.ID)
	}

	resp := &taskqueuev1.CreateTaskResponse{
		Name:       taskName,
		CreateTime: time.Now().Format(time.RFC3339),
	}
	if scheduleTime != nil {
		resp.ScheduleTime = scheduleTime.Format(time.RFC3339)
	}

	respBytes, err := pjson.Marshal(resp)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBytes); err != nil {
		slog.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

func (h *Handler) CreateTaskWithQueue(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "queue")
	if queueName == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "queue name is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("failed to read request body: %v", err))
		return
	}

	var req taskqueuev1.CreateTaskRequest
	if err := pjson.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if err := pjson.Validate(&req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("validation error: %v", err))
		return
	}

	decodedBody, err := base64.StdEncoding.DecodeString(req.Task.HttpRequest.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid base64 body: %v", err))
		return
	}

	payload := queue.NewTaskPayload(decodedBody, req.Task.HttpRequest.Headers)

	// Inject trace context (traceparent/tracestate) into task headers
	tracing.InjectToMap(r.Context(), payload.Headers)

	// Inject x-request-id into task headers
	reqID := logging.RequestIDFromContext(r.Context())
	if reqID == "" {
		reqID = logging.ValidateAndExtractRequestID("")
	}
	payload.Headers["x-request-id"] = reqID

	var scheduleTime *time.Time
	if req.Task.ScheduleTime != "" {
		t, err := time.Parse(time.RFC3339, req.Task.ScheduleTime)
		if err != nil {
			WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid scheduleTime format: %v", err))
			return
		}
		scheduleTime = &t
	}

	info, err := h.client.EnqueueTaskWithQueue(payload, scheduleTime, queueName, req.Task.Name)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			WriteError(w, http.StatusConflict, StatusAlreadyExists, fmt.Sprintf("task with name %q already exists", req.Task.Name))
			return
		}
		slog.ErrorContext(r.Context(), "failed to enqueue task",
			slog.String("event", "task.enqueue.fail"),
			slog.String("error", err.Error()),
			slog.String("queue", queueName),
		)
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to enqueue task")
		return
	}

	taskName := req.Task.Name
	if taskName == "" {
		taskName = fmt.Sprintf("tasks/%s", info.ID)
	}

	resp := &taskqueuev1.CreateTaskResponse{
		Name:       taskName,
		CreateTime: time.Now().Format(time.RFC3339),
	}
	if scheduleTime != nil {
		resp.ScheduleTime = scheduleTime.Format(time.RFC3339)
	}

	respBytes, err := pjson.Marshal(resp)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBytes); err != nil {
		slog.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

// HealthCheck returns a simple health check response for backward compatibility.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

// HealthLive returns a simple health check response for liveness probes.
func (h *Handler) HealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

// HealthReady returns a health check response for readiness probes.
// It verifies that the Redis connection is healthy.
func (h *Handler) HealthReady(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"status":  "healthy",
			"version": version,
			"checks": map[string]any{
				"redis": map[string]string{"status": "healthy"},
			},
		}

		// Check Redis connectivity via Asynq inspector
		if err := h.client.Ping(); err != nil {
			response["status"] = "unhealthy"
			response["checks"] = map[string]any{
				"redis": map[string]any{
					"status": "unhealthy",
					"error":  err.Error(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				slog.Warn("failed to write response", slog.String("error", err.Error()))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Warn("failed to write response", slog.String("error", err.Error()))
		}
	}
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "task ID is required")
		return
	}

	h.deleteTaskFromQueue(r.Context(), w, h.client.DefaultQueueName(), taskID)
}

func (h *Handler) DeleteTaskWithQueue(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "queue")
	if queueName == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "queue name is required")
		return
	}

	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "task ID is required")
		return
	}

	h.deleteTaskFromQueue(r.Context(), w, queueName, taskID)
}

func (h *Handler) deleteTaskFromQueue(ctx context.Context, w http.ResponseWriter, queueName, taskID string) {
	err := h.client.DeleteTaskFromQueue(queueName, taskID)
	if err != nil {
		if errors.Is(err, asynq.ErrQueueNotFound) || errors.Is(err, asynq.ErrTaskNotFound) {
			WriteError(w, http.StatusNotFound, StatusNotFound,
				fmt.Sprintf("task %q not found in queue %q", taskID, queueName))
			return
		}

		slog.ErrorContext(ctx, "failed to delete task",
			slog.String("event", "task.delete.fail"),
			slog.String("error", err.Error()),
			slog.String("queue", queueName),
			slog.String("task_id", taskID),
		)
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to delete task")
		return
	}

	resp := &taskqueuev1.DeleteTaskResponse{}
	respBytes, err := pjson.Marshal(resp)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBytes); err != nil {
		slog.Warn("failed to write response", slog.String("error", err.Error()))
	}
}
