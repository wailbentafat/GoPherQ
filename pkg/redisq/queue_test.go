package redisq_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wailbentafat/go-worker-pool/pkg/redisq"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	rawRedisClient    *redis.Client
	testredisaddresse string 

	defaultTestQueue = "test_tasks_queue" 
)

func TestMain(m *testing.M) {
	testRedisAddress := os.Getenv("TEST_REDIS_ADDRESS")
	if testRedisAddress == "" {
		testredisaddresse = "localhost:6379"
	} else {
		testredisaddresse = testRedisAddress
	}

	rawRedisClient = redis.NewClient(
		&redis.Options{
			Addr: testredisaddresse,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rawRedisClient.Ping(ctx).Err(); err != nil {
		panic("Could not connect to test Redis: " + err.Error()) 
	}
	exitCode := m.Run()
	if rawRedisClient != nil {
		rawRedisClient.Close()
	}
	os.Exit(exitCode)
}

func flushQueue(t *testing.T, queueName string) {
	t.Helper()
	err := rawRedisClient.Del(context.Background(), queueName).Err()
	require.NoError(t, err, "Failed to flush queue %s", queueName)
}



func TestNewClient(t *testing.T) {
	opts := &redis.Options{
		Addr: testredisaddresse,
	}
	client, err := redisq.NewClient(opts, defaultTestQueue)
	require.NoError(t, err, "NewClient should not return an error with valid options")
	require.NotNil(t, client, "NewClient should return a non-nil client")
}

func TestEnqueueAndDequeue(t *testing.T) {
	opts := &redis.Options{
		Addr: testredisaddresse,
	}
	client, err := redisq.NewClient(opts, defaultTestQueue)
	require.NoError(t, err, "Failed to create client for EnqueueDequeue test")
	require.NotNil(t, client)

	flushQueue(t, defaultTestQueue)
	taskToEnqueue := redisq.TaskPayload{
		TaskID:  "test-task-123",
		Header:  "test-header",
		Body:    "test-body",
		Methode: "POST",
	}
	err = client.Enqueue(taskToEnqueue)
	require.NoError(t, err, "Enqueue failed")
	dequeuedTask, err := client.Dequeue()
	require.NoError(t, err, "Dequeue failed")
	assert.Equal(t, taskToEnqueue.TaskID, dequeuedTask.TaskID, "TaskID mismatch")
	assert.Equal(t, taskToEnqueue.Header, dequeuedTask.Header, "Header mismatch")
	assert.Equal(t, taskToEnqueue.Body, dequeuedTask.Body, "Body mismatch")
	assert.Equal(t, taskToEnqueue.Methode, dequeuedTask.Methode, "Methode mismatch")
	_, err = client.Dequeue() 
	require.Error(t, err, "Dequeue on an empty queue should return an error")
	assert.Contains(t, err.Error(), "queue is empty", "Expected 'queue is empty' error")
}

func TestDequeue_EmptyQueue(t *testing.T) {
	opts := &redis.Options{
		Addr: testredisaddresse,
	}
	emptyQueueName := "test_empty_queue_for_dequeue"
	client, err := redisq.NewClient(opts, emptyQueueName)
	require.NoError(t, err)
	require.NotNil(t, client)

	flushQueue(t, emptyQueueName)

	_, err = client.Dequeue()
	require.Error(t, err, "Dequeue on an empty queue should return an error")
	assert.Contains(t, err.Error(), "queue is empty", "Expected 'queue is empty' error from Dequeue")
}
func TestEnqueueWithDelay(t *testing.T) {
	opts := &redis.Options{
		Addr: testredisaddresse,
	}
	delayQueueName := "test_delay_queue"
	client, err := redisq.NewClient(opts, delayQueueName)
	require.NoError(t, err)
	require.NotNil(t, client)

	flushQueue(t, delayQueueName)

	taskToDelay := redisq.TaskPayload{
		TaskID:  "delayed-task-001",
		Header:  "delay-header",
		Body:    "delay-body",
		Methode: "GET",
	}

	delaySeconds := int64(1) 

	err = client.EnqueueWithDelay(taskToDelay, delaySeconds)
	require.NoError(t, err, "EnqueueWithDelay returned an error")

	_, err = client.Dequeue()
	require.Error(t, err, "Dequeue immediately after EnqueueWithDelay should fail or timeout")
	assert.Contains(t, err.Error(), "queue is empty")
	time.Sleep(time.Duration(delaySeconds+1) * time.Second)
	dequeuedTask, err := client.Dequeue()
	require.NoError(t, err, "Dequeue after delay failed")
	assert.Equal(t, taskToDelay.TaskID, dequeuedTask.TaskID)
}