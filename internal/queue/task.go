package queue

import (
	"encoding/json"
	"time"
)

const TaskTypeHTTPForward = "http:forward"

type TaskPayload struct {
	Body      []byte            `json:"body"`
	Headers   map[string]string `json:"headers"`
	CreatedAt time.Time         `json:"created_at"`
}

func NewTaskPayload(body []byte, headers map[string]string) *TaskPayload {
	return &TaskPayload{
		Body:      body,
		Headers:   headers,
		CreatedAt: time.Now(),
	}
}

func (p *TaskPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func UnmarshalTaskPayload(data []byte) (*TaskPayload, error) {
	var p TaskPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
