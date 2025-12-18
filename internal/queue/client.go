package queue

import (
	"time"

	"github.com/hibiken/asynq"

	"github.com/KasumiMercury/primind-tasks/internal/config"
)

type Client struct {
	client     *asynq.Client
	queueName  string
	retryCount int
}

func NewClient(cfg *config.Config) *Client {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &Client{
		client:     client,
		queueName:  cfg.QueueName,
		retryCount: cfg.RetryCount,
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) EnqueueTask(payload *TaskPayload, scheduleTime *time.Time) (*asynq.TaskInfo, error) {
	return c.EnqueueTaskWithQueue(payload, scheduleTime, c.queueName)
}

func (c *Client) EnqueueTaskWithQueue(payload *TaskPayload, scheduleTime *time.Time, queueName string) (*asynq.TaskInfo, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, err
	}

	task := asynq.NewTask(TaskTypeHTTPForward, data)

	opts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.MaxRetry(c.retryCount),
	}

	if scheduleTime != nil && scheduleTime.After(time.Now()) {
		opts = append(opts, asynq.ProcessAt(*scheduleTime))
	}

	return c.client.Enqueue(task, opts...)
}
