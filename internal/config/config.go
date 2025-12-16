package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	TargetEndpoint    string
	RetryCount        int
	APIPort           int
	WorkerConcurrency int
	QueueName         string
	RequestTimeout    time.Duration
}

func Load() *Config {
	return &Config{
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvInt("REDIS_DB", 0),
		TargetEndpoint:    getEnv("TARGET_ENDPOINT", ""),
		RetryCount:        getEnvInt("RETRY_COUNT", 3),
		APIPort:           getEnvInt("API_PORT", 8080),
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 10),
		QueueName:         getEnv("QUEUE_NAME", "default"),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
