package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynq"

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
	payload, err := queue.UnmarshalTaskPayload(t.Payload())
	if err != nil {
		log.Printf("failed to unmarshal payload: %v", err)
		return fmt.Errorf("unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.targetEndpoint, bytes.NewReader(payload.Body))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		return fmt.Errorf("create request: %w: %w", err, asynq.SkipRetry)
	}

	for k, v := range payload.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("http request failed: %v", err)
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("task completed successfully: status=%d", resp.StatusCode)
		return nil
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		log.Printf("client error (no retry): status=%d, body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("client error %d: %w", resp.StatusCode, asynq.SkipRetry)
	}

	log.Printf("server error (will retry): status=%d, body=%s", resp.StatusCode, string(body))
	return fmt.Errorf("server error %d", resp.StatusCode)
}
