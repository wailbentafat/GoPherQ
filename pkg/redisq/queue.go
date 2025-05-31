package redisq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	// Step 1: Import your task package
	"github.com/wailbentafat/go-worker-pool/pkg/task" // Make sure this path is correct
)

// NOTE: Any local 'type TaskPayload struct { ... }' definition that might have existed in this file
// should be REMOVED. We are now using task.ApiTaskPayload from the imported task package.

// Step 2: Update Client interface to use task.ApiTaskPayload
type Client interface {
	Enqueue(payload task.ApiTaskPayload) error
	Dequeue() (task.ApiTaskPayload, error)
	EnqueueWithDelay(payload task.ApiTaskPayload, delayInSeconds int64) error
	// EnqueueWithDelayAndTimeout(payload task.ApiTaskPayload, delay int64, timeout int64) error
	// EnqueueWithTimeout(payload task.ApiTaskPayload, timeout int64) error
}

type redisClient struct {
	rdb       *redis.Client
	queueName string
}

func NewClient(opt *redis.Options, queueName string) (Client, error) {
	rdb := redis.NewClient(opt)
	// Use a context with timeout for the initial Ping
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &redisClient{
		rdb:       rdb,
		queueName: queueName,
	}, nil
}

// Step 3: Update method implementations to use task.ApiTaskPayload
func (c *redisClient) Enqueue(payload task.ApiTaskPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task.ApiTaskPayload: %w", err)
	}
	// Consider passing context as a parameter here instead of context.TODO()
	err = c.rdb.RPush(context.TODO(), c.queueName, payloadBytes).Err()
	if err != nil {
		return fmt.Errorf("failed to RPush task to queue %s: %w", c.queueName, err)
	}
	// Use the 'Id' field from task.ApiTaskPayload
	fmt.Printf("Task enqueued to %s: %s\n", c.queueName, payload.Id)
	return nil
}

func (c *redisClient) Dequeue() (task.ApiTaskPayload, error) {
	var dequeuedTask task.ApiTaskPayload // Use task.ApiTaskPayload here

	// Consider passing context as a parameter here instead of context.TODO()
	resultSlice, err := c.rdb.BRPop(context.TODO(), 5*time.Second, c.queueName).Result()
	if err != nil {
		if err == redis.Nil { // redis.Nil indicates a timeout (queue was empty)
			return task.ApiTaskPayload{}, fmt.Errorf("queue '%s' is empty after timeout", c.queueName)
		}
		return task.ApiTaskPayload{}, fmt.Errorf("failed to BRPop task from queue %s: %w", c.queueName, err)
	}

	if len(resultSlice) != 2 { // BRPop returns [keyName, value]
		return task.ApiTaskPayload{}, fmt.Errorf("unexpected result format from BRPop for queue %s: expected 2 elements, got %d. Result: %v", c.queueName, len(resultSlice), resultSlice)
	}

	taskDataBytes := []byte(resultSlice[1]) // The actual payload

	if err := json.Unmarshal(taskDataBytes, &dequeuedTask); err != nil {
		// Error message now correctly reflects we expect task.ApiTaskPayload
		return task.ApiTaskPayload{}, fmt.Errorf("failed to unmarshal task.ApiTaskPayload from queue %s: %w. Raw data: %s", c.queueName, err, string(taskDataBytes))
	}

	// Use the 'Id' field from task.ApiTaskPayload
	fmt.Printf("Task dequeued from %s: %s\n", c.queueName, dequeuedTask.Id)
	return dequeuedTask, nil
}

func (c *redisClient) EnqueueWithDelay(payload task.ApiTaskPayload, delayInSeconds int64) error {
	if delayInSeconds <= 0 {
		// It's generally better to return an error for invalid input
		// rather than trying to "fix" it by enqueuing immediately,
		// unless that's the desired behavior.
		return fmt.Errorf("delay must be a positive number of seconds")
	}

	go func() {
		time.Sleep(time.Duration(delayInSeconds) * time.Second)
		// payload here is already task.ApiTaskPayload
		if err := c.Enqueue(payload); err != nil {
			// Log errors from background goroutines carefully
			fmt.Printf("Error: failed to enqueue delayed task %s (ID: %s): %v\n", payload.Id, payload.Id, err)
		}
	}()
	return nil
}