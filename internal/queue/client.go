package queue

import (
	"log"
	"time"

	"github.com/hibiken/asynq"

	"github.com/KasumiMercury/primind-tasks/internal/config"
)

type Client struct {
	client     *asynq.Client
	inspector  *asynq.Inspector
	queueName  string
	retryCount int
}

func NewClient(cfg *config.Config) *Client {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	return &Client{
		client:     asynq.NewClient(redisOpt),
		inspector:  asynq.NewInspector(redisOpt),
		queueName:  cfg.QueueName,
		retryCount: cfg.RetryCount,
	}
}

func (c *Client) Close() error {
	if err := c.inspector.Close(); err != nil {
		return err
	}
	return c.client.Close()
}

func (c *Client) DefaultQueueName() string {
	return c.queueName
}

func (c *Client) DeleteTask(taskID string) error {
	return c.DeleteTaskFromQueue(c.queueName, taskID)
}

func (c *Client) DeleteTaskFromQueue(queueName, taskID string) error {
	info, err := c.inspector.GetTaskInfo(queueName, taskID)
	if err != nil {
		return err
	}

	if info.State == asynq.TaskStateActive {
		if err := c.inspector.CancelProcessing(taskID); err != nil {
			log.Printf("warning: could not cancel active task %s: %v", taskID, err)
		}
		return nil
	}

	return c.inspector.DeleteTask(queueName, taskID)
}

func (c *Client) EnqueueTask(payload *TaskPayload, scheduleTime *time.Time, taskID string) (*asynq.TaskInfo, error) {
	return c.EnqueueTaskWithQueue(payload, scheduleTime, c.queueName, taskID)
}

func (c *Client) EnqueueTaskWithQueue(payload *TaskPayload, scheduleTime *time.Time, queueName string, taskID string) (*asynq.TaskInfo, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, err
	}

	task := asynq.NewTask(TaskTypeHTTPForward, data)

	opts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.MaxRetry(c.retryCount),
	}

	if taskID != "" {
		opts = append(opts, asynq.TaskID(taskID))
	}

	if scheduleTime != nil && scheduleTime.After(time.Now()) {
		opts = append(opts, asynq.ProcessAt(*scheduleTime))
	}

	return c.client.Enqueue(task, opts...)
}

// Ping checks if the Redis connection is healthy by listing queues.
func (c *Client) Ping() error {
	_, err := c.inspector.Queues()
	return err
}
