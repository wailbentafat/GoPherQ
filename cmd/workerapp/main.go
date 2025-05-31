package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/wailbentafat/GoPherQ/pkg/executor"
	"github.com/wailbentafat/GoPherQ/pkg/redisq"
	"github.com/wailbentafat/GoPherQ/pkg/workerpool"
)

const (
	redisAddressEnvVar  = "REDIS_ADDRESS"
	redisPasswordEnvVar = "REDIS_PASSWORD"
	redisDBEnvVar       = "REDIS_DB"
	queueNameEnvVar     = "QUEUE_NAME"
	numWorkersEnvVar    = "NUM_WORKERS"

	defaultRedisAddress = "localhost:6379"
	defaultQueueName    = "my_tasks_queue"
	defaultNumWorkers   = 5
)

func main() {
	log.Println("Starting worker application...")

	redisAddr := getEnv(redisAddressEnvVar, defaultRedisAddress)
	redisPassword := getEnv(redisPasswordEnvVar, "")
	redisDB := parseIntEnv(redisDBEnvVar, 0)
	queueName := getEnv(queueNameEnvVar, defaultQueueName)
	numWorkers := parseIntEnv(numWorkersEnvVar, defaultNumWorkers)

	log.Printf("Configuration: RedisAddr=%s, QueueName=%s, NumWorkers=%d", redisAddr, queueName, numWorkers)

	redisOpts := &redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	}
	redisQueueClient, err := redisq.NewClient(redisOpts, queueName)
	if err != nil {
		log.Fatalf("Failed to initialize Redis queue client: %v", err)
	}

	apiExecutor := executor.NewApiTaskExecutor(nil)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	pool := workerpool.NewWorkerPool(appCtx, apiExecutor, int64(numWorkers))
	pool.Work()
	log.Printf("Worker pool started with %d workers, listening on queue '%s'", numWorkers, queueName)
	go func() {
		for {
			select {
			case <-appCtx.Done():
				log.Println("Task dequeuing loop: Application context cancelled, stopping task fetching.")
				return
			default:
				log.Printf("Task dequeuing loop: Waiting for task from queue '%s'...\n", queueName)
				dequeuedTask, err := redisQueueClient.Dequeue()
				if err != nil {
					if err.Error() == fmt.Sprintf("queue '%s' is empty after timeout", queueName) || err.Error() == "queue is empty" {
						continue
					}
					log.Printf("Task dequeuing loop: Error dequeuing task: %v. Retrying in 5 seconds...", err)
					select {
					case <-time.After(5 * time.Second):
					case <-appCtx.Done():
						log.Println("Task dequeuing loop: Context cancelled during retry delay.")
						return
					}
					continue
				}

				pool.Send(dequeuedTask)
			}
		}
	}()

	// pool.CancelOnsignal(syscall.SIGINT, syscall.SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %s. Shutting down gracefully...", sig)
		appCancel()
	case <-appCtx.Done():
		log.Println("Application context done. Shutting down...")
	}

	log.Println("Attempting to close worker pool...")
	if err := pool.Close(); err != nil {
		log.Printf("Error during worker pool close: %v", err)
	} else {
		log.Println("Worker pool closed successfully.")
	}

	log.Println("Worker application shut down.")
}
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parseIntEnv(key string, fallback int) int {
	if valueStr, ok := os.LookupEnv(key); ok {
		if valueInt, err := time.ParseDuration(valueStr); err == nil {
			log.Printf("Warning: parseIntEnv using ParseDuration for %s, use strconv.Atoi for general integers", key)
			fmt.Print(valueInt)
		}
		if valueInt, err := StrToInt(valueStr); err == nil {
			return valueInt
		} else {
			log.Printf("Warning: Invalid value for %s: %s. Using default %d. Error: %v", key, valueStr, fallback, err)
		}
	}
	return fallback
}

func StrToInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscan(s, &i)
	return i, err
}
