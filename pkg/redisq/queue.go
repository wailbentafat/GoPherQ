package redisq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/wailbentafat/GoPherQ/pkg/task" 
)


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

func (c *redisClient) Enqueue(payload task.ApiTaskPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task.ApiTaskPayload: %w", err)
	}
	err = c.rdb.RPush(context.TODO(), c.queueName, payloadBytes).Err()
	if err != nil {
		return fmt.Errorf("failed to RPush task to queue %s: %w", c.queueName, err)
	}
	fmt.Printf("Task enqueued to %s: %s\n", c.queueName, payload.Id)
	return nil
}

func (c *redisClient) Dequeue() (task.ApiTaskPayload, error) {
	var dequeuedTask task.ApiTaskPayload
	resultSlice, err := c.rdb.BRPop(context.TODO(), 5*time.Second, c.queueName).Result()
	if err != nil {
		if err == redis.Nil { 
			return task.ApiTaskPayload{}, fmt.Errorf("queue '%s' is empty after timeout", c.queueName)
		}
		return task.ApiTaskPayload{}, fmt.Errorf("failed to BRPop task from queue %s: %w", c.queueName, err)
	}

	if len(resultSlice) != 2 {
		return task.ApiTaskPayload{}, fmt.Errorf("unexpected result format from BRPop for queue %s: expected 2 elements, got %d. Result: %v", c.queueName, len(resultSlice), resultSlice)
	}

	taskDataBytes := []byte(resultSlice[1]) 

	if err := json.Unmarshal(taskDataBytes, &dequeuedTask); err != nil {
		return task.ApiTaskPayload{}, fmt.Errorf("failed to unmarshal task.ApiTaskPayload from queue %s: %w. Raw data: %s", c.queueName, err, string(taskDataBytes))
	}

	fmt.Printf("Task dequeued from %s: %s\n", c.queueName, dequeuedTask.Id)
	return dequeuedTask, nil
}

func (c *redisClient) EnqueueWithDelay(payload task.ApiTaskPayload, delayInSeconds int64) error {
	if delayInSeconds <= 0 {
	
		return fmt.Errorf("delay must be a positive number of seconds")
	}

	go func() {
		time.Sleep(time.Duration(delayInSeconds) * time.Second)
		if err := c.Enqueue(payload); err != nil {
			fmt.Printf("Error: failed to enqueue delayed task %s (ID: %s): %v\n", payload.Id, payload.Id, err)
		}
	}()
	return nil
}