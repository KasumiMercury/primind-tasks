package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Task.HTTPRequest == nil {
		http.Error(w, "httpRequest is required", http.StatusBadRequest)
		return
	}

	body, err := req.Task.HTTPRequest.DecodeBody()
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid base64 body: %v", err), http.StatusBadRequest)
		return
	}

	payload := queue.NewTaskPayload(body, req.Task.HTTPRequest.Headers)

	var scheduleTime *time.Time
	if req.Task.ScheduleTime != "" {
		t, err := time.Parse(time.RFC3339, req.Task.ScheduleTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid scheduleTime format: %v", err), http.StatusBadRequest)
			return
		}
		scheduleTime = &t
	}

	info, err := h.client.EnqueueTask(payload, scheduleTime)
	if err != nil {
		log.Printf("failed to enqueue task: %v", err)
		http.Error(w, "failed to enqueue task", http.StatusInternalServerError)
		return
	}

	resp := cloudtasks.CreateTaskResponse{
		Name:       fmt.Sprintf("tasks/%s", info.ID),
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
		http.Error(w, "queue name is required", http.StatusBadRequest)
		return
	}

	var req cloudtasks.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Task.HTTPRequest == nil {
		http.Error(w, "httpRequest is required", http.StatusBadRequest)
		return
	}

	body, err := req.Task.HTTPRequest.DecodeBody()
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid base64 body: %v", err), http.StatusBadRequest)
		return
	}

	payload := queue.NewTaskPayload(body, req.Task.HTTPRequest.Headers)

	var scheduleTime *time.Time
	if req.Task.ScheduleTime != "" {
		t, err := time.Parse(time.RFC3339, req.Task.ScheduleTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid scheduleTime format: %v", err), http.StatusBadRequest)
			return
		}
		scheduleTime = &t
	}

	info, err := h.client.EnqueueTaskWithQueue(payload, scheduleTime, queueName)
	if err != nil {
		log.Printf("failed to enqueue task: %v", err)
		http.Error(w, "failed to enqueue task", http.StatusInternalServerError)
		return
	}

	resp := cloudtasks.CreateTaskResponse{
		Name:       fmt.Sprintf("tasks/%s", info.ID),
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
