package cloudtasks

import (
	"encoding/base64"
	"fmt"
)

type CreateTaskRequest struct {
	Task Task `json:"task"`
}

type Task struct {
	Name         string       `json:"name,omitempty"`
	HTTPRequest  *HTTPRequest `json:"httpRequest,omitempty"`
	ScheduleTime string       `json:"scheduleTime,omitempty"`
	CreateTime   string       `json:"createTime,omitempty"`
}

type HTTPRequest struct {
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type CreateTaskResponse struct {
	Name         string `json:"name"`
	ScheduleTime string `json:"scheduleTime,omitempty"`
	CreateTime   string `json:"createTime"`
}

func (r *HTTPRequest) DecodeBody() ([]byte, error) {
	if r.Body == "" {
		return []byte{}, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 body: %w", err)
	}
	return decoded, nil
}
