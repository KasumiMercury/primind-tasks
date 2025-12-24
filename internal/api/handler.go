package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	taskqueuev1 "github.com/KasumiMercury/primind-tasks/internal/gen/taskqueue/v1"
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
	w.Write(respBytes)
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
	w.Write(respBytes)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, StatusInvalidArgument, "task ID is required")
		return
	}

	h.deleteTaskFromQueue(w, h.client.DefaultQueueName(), taskID)
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

	h.deleteTaskFromQueue(w, queueName, taskID)
}

func (h *Handler) deleteTaskFromQueue(w http.ResponseWriter, queueName, taskID string) {
	err := h.client.DeleteTaskFromQueue(queueName, taskID)
	if err != nil {
		if errors.Is(err, asynq.ErrQueueNotFound) || errors.Is(err, asynq.ErrTaskNotFound) {
			WriteError(w, http.StatusNotFound, StatusNotFound,
				fmt.Sprintf("task %q not found in queue %q", taskID, queueName))
			return
		}

		log.Printf("failed to delete task: %v", err)
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
	w.Write(respBytes)
}
