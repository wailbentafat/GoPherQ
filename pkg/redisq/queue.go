package redisq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)




type TaskPayload struct {
	TaskID string `json:"task_id"`
    Header string `json:"header"`
	Body   string `json:"body"`
	Methode string `json:"methode"`
}
type Client interface{
	Enqueue(payload TaskPayload) error
	Dequeue() (TaskPayload, error) 
	EnqueueWithDelay(payload TaskPayload, delay int64) error
	// EnqueueWithDelayAndTimeout(payload TaskPayload, delay int64, timeout int64) error
	// EnqueueWithTimeout(payload TaskPayload, timeout int64) error
}
type redisClient struct{
	rdb *redis.Client
	queueName string
}
func NewClient (opt *redis.Options, queueName string) (Client,error) {
	rdb:= redis.NewClient(opt)
	if err := rdb.Ping(rdb.Context()).Err(); err != nil {
		return nil, err
	}
	return &redisClient{
		rdb: rdb,
		queueName: queueName,
	}, nil

}
func (c *redisClient) Enqueue(payload TaskPayload) error {
	payloadBytes,err:=json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	err=c.rdb.RPush(context.TODO(),c.queueName,payloadBytes).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	fmt.Printf("Task enqueued: %s\n", payload.TaskID)
	return nil
}

func (c *redisClient)Dequeue() (TaskPayload, error) {
	var task TaskPayload
	payloadBytes, err := c.rdb.BRPop(context.TODO(),5*time.Second, c.queueName).Result()
	 if err!=nil {
		if err == redis.Nil {
			return TaskPayload{}, fmt.Errorf("queue is empty")
		}
		return TaskPayload{}, fmt.Errorf("failed to dequeue task: %w", err)
	 }
	 if len(payloadBytes)==2{
		return task, nil
	 }
	 results:=[]byte(payloadBytes[1])
	 if err:=json.Unmarshal(results, &task); err != nil {

		return task, fmt.Errorf("failed to unmarshal payload: %w", err)
	 }
	 fmt.Printf("Task dequeued: %s\n", task.TaskID)
	return task, nil

}
func (c *redisClient) EnqueueWithDelay(payload TaskPayload, delay int64) error {
	 if delay<= 0 {
		return fmt.Errorf("delay must be greater than zero")
	 }
	 go func(){
		time.Sleep(time.Duration(delay) * time.Second)
		c.Enqueue(payload)
	 }()
	 return nil
}

