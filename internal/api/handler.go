package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	"github.com/KasumiMercury/primind-tasks/internal/queue"
	"github.com/KasumiMercury/primind-tasks/pkg/cloudtasks"
)

type Handler struct {
	client *queue.Client
}

func NewHandler(client *queue.Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req cloudtasks.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Task.HTTPRequest == nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "httpRequest is required")
		return
	}

	body, err := req.Task.HTTPRequest.DecodeBody()
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid base64 body: %v", err))
		return
	}

	payload := queue.NewTaskPayload(body, req.Task.HTTPRequest.Headers)

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
		log.Printf("failed to enqueue task: %v", err)
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to enqueue task")
		return
	}

	taskName := req.Task.Name
	if taskName == "" {
		taskName = fmt.Sprintf("tasks/%s", info.ID)
	}

	resp := cloudtasks.CreateTaskResponse{
		Name:       taskName,
		CreateTime: time.Now().Format(time.RFC3339),
	}
	if scheduleTime != nil {
		resp.ScheduleTime = scheduleTime.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) CreateTaskWithQueue(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "queue")
	if queueName == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "queue name is required")
		return
	}

	var req cloudtasks.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Task.HTTPRequest == nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "httpRequest is required")
		return
	}

	body, err := req.Task.HTTPRequest.DecodeBody()
	if err != nil {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, fmt.Sprintf("invalid base64 body: %v", err))
		return
	}

	payload := queue.NewTaskPayload(body, req.Task.HTTPRequest.Headers)

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
		log.Printf("failed to enqueue task: %v", err)
		WriteError(w, http.StatusInternalServerError, StatusInternal, "failed to enqueue task")
		return
	}

	taskName := req.Task.Name
	if taskName == "" {
		taskName = fmt.Sprintf("tasks/%s", info.ID)
	}

	resp := cloudtasks.CreateTaskResponse{
		Name:       taskName,
		CreateTime: time.Now().Format(time.RFC3339),
	}
	if scheduleTime != nil {
		resp.ScheduleTime = scheduleTime.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
