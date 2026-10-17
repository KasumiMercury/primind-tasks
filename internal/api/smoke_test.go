package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/KasumiMercury/primind-tasks/internal/api"
	"github.com/KasumiMercury/primind-tasks/internal/config"
	"github.com/KasumiMercury/primind-tasks/internal/queue"
	"github.com/KasumiMercury/primind-tasks/internal/worker"
)

type delivery struct {
	body      string
	requestID string
}

type webhook struct {
	mu         sync.Mutex
	deliveries []delivery
}

func (wh *webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	wh.mu.Lock()
	wh.deliveries = append(wh.deliveries, delivery{body: string(body), requestID: r.Header.Get("x-request-id")})
	wh.mu.Unlock()

	if strings.HasPrefix(string(body), "reject") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (wh *webhook) find(body string) []delivery {
	wh.mu.Lock()
	defer wh.mu.Unlock()

	var found []delivery
	for _, d := range wh.deliveries {
		if d.body == body {
			found = append(found, d)
		}
	}
	return found
}

type env struct {
	baseURL   string
	cfg       *config.Config
	webhook   *webhook
	inspector *asynq.Inspector
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()

	redisC, err := tcredis.Run(ctx, "redis:8-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = redisC.Terminate(context.Background()) })

	redisAddr, err := redisC.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("redis endpoint: %v", err)
	}

	wh := &webhook{}
	stub := httptest.NewServer(wh)
	t.Cleanup(stub.Close)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	t.Setenv("REDIS_ADDR", redisAddr)
	t.Setenv("TARGET_ENDPOINT", stub.URL)
	t.Setenv("API_PORT", fmt.Sprint(port))
	t.Setenv("QUEUE_NAME", "default")
	t.Setenv("RETRY_COUNT", "3")
	cfg := config.Load()

	client := queue.NewClient(cfg)
	t.Cleanup(func() { _ = client.Close() })

	server := api.NewServer(cfg, client, "test")
	go func() { _ = server.ListenAndServe() }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	w := worker.NewServer(cfg)
	go func() { _ = w.Run() }()
	t.Cleanup(w.Shutdown)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	t.Cleanup(func() { _ = inspector.Close() })

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	eventually(t, "api server ready", func() bool {
		resp, err := http.Get(baseURL + "/health/ready")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	return &env{baseURL: baseURL, cfg: cfg, webhook: wh, inspector: inspector}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (e *env) do(t *testing.T, method, path, body string, header http.Header) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody)
}

func createBody(name, payload, scheduleTime string) string {
	return fmt.Sprintf(`{"task":{"name":%q,"http_request":{"body":%q},"schedule_time":%q}}`,
		name, base64.StdEncoding.EncodeToString([]byte(payload)), scheduleTime)
}

func TestSmoke(t *testing.T) {
	e := setup(t)

	t.Run("created task is delivered to the webhook with x-request-id", func(t *testing.T) {
		requestID := uuid.Must(uuid.NewV7()).String()
		payload := "hello " + requestID

		status, body := e.do(t, http.MethodPost, "/tasks", createBody("", payload, ""),
			http.Header{"X-Request-Id": {requestID}})
		if status != http.StatusOK {
			t.Fatalf("create: got %d %s", status, body)
		}

		eventually(t, "webhook delivery", func() bool { return len(e.webhook.find(payload)) > 0 })
		if got := e.webhook.find(payload)[0].requestID; got != requestID {
			t.Fatalf("x-request-id: got %q, want %q", got, requestID)
		}
	})

	t.Run("duplicate task name returns 409 and the task can be deleted", func(t *testing.T) {
		name := "dup-" + uuid.NewString()
		later := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

		if status, body := e.do(t, http.MethodPost, "/tasks", createBody(name, "dup", later), nil); status != http.StatusOK {
			t.Fatalf("first create: got %d %s", status, body)
		}
		if status, body := e.do(t, http.MethodPost, "/tasks", createBody(name, "dup", later), nil); status != http.StatusConflict {
			t.Fatalf("second create: got %d %s, want 409", status, body)
		}
		if status, body := e.do(t, http.MethodDelete, "/tasks/"+name, "", nil); status != http.StatusOK {
			t.Fatalf("delete: got %d %s", status, body)
		}
	})

	t.Run("deleting a missing task returns 404", func(t *testing.T) {
		if status, body := e.do(t, http.MethodDelete, "/tasks/missing-"+uuid.NewString(), "", nil); status != http.StatusNotFound {
			t.Fatalf("got %d %s, want 404", status, body)
		}
	})

	t.Run("deleting from a missing queue returns 404", func(t *testing.T) {
		if status, body := e.do(t, http.MethodDelete, "/tasks/no-such-queue/some-task", "", nil); status != http.StatusNotFound {
			t.Fatalf("got %d %s, want 404", status, body)
		}
	})

	t.Run("invalid request returns 400", func(t *testing.T) {
		for _, body := range []string{`not json`, `{"task":{}}`} {
			if status, resp := e.do(t, http.MethodPost, "/tasks", body, nil); status != http.StatusBadRequest {
				t.Fatalf("%s: got %d %s, want 400", body, status, resp)
			}
		}
	})

	t.Run("webhook 4xx archives the task without retry", func(t *testing.T) {
		name := "reject-" + uuid.NewString()
		payload := "reject " + name

		if status, body := e.do(t, http.MethodPost, "/tasks", createBody(name, payload, ""), nil); status != http.StatusOK {
			t.Fatalf("create: got %d %s", status, body)
		}

		var info *asynq.TaskInfo
		eventually(t, "task archived", func() bool {
			var err error
			info, err = e.inspector.GetTaskInfo(e.cfg.QueueName, name)
			return err == nil && info.State == asynq.TaskStateArchived
		})
		if info.Retried != 0 {
			t.Fatalf("retried %d times, want 0", info.Retried)
		}
		if n := len(e.webhook.find(payload)); n != 1 {
			t.Fatalf("webhook called %d times, want 1", n)
		}
	})

	t.Run("grpc health check reports SERVING over h2c", func(t *testing.T) {
		protocols := new(http.Protocols)
		protocols.SetUnencryptedHTTP2(true)
		h2c := &http.Client{Transport: &http.Transport{Protocols: protocols}}

		req, err := http.NewRequest(http.MethodPost, e.baseURL+"/grpc.health.v1.Health/Check",
			bytes.NewReader([]byte{0, 0, 0, 0, 0}))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/grpc")
		req.Header.Set("Te", "trailers")

		resp, err := h2c.Do(req)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.ProtoMajor != 2 {
			t.Fatalf("protocol: got %s, want HTTP/2", resp.Proto)
		}
		if got := resp.Trailer.Get("Grpc-Status"); got != "0" {
			t.Fatalf("grpc-status: got %q, want 0 (message %q)", got, resp.Trailer.Get("Grpc-Message"))
		}
		if want := []byte{0, 0, 0, 0, 2, 0x08, 0x01}; !bytes.Equal(body, want) {
			t.Fatalf("response frame: got %x, want %x (status SERVING)", body, want)
		}
	})
}
