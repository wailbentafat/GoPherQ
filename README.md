# GoPherQ 🐿️+📦

**GoPherQ** is a Go-based distributed task queue system, inspired by Celery, designed to execute tasks asynchronously using Redis as a message broker.

## Current Status

GoPherQ is currently in active development, progressing through **Phase 1 towards v1.0**. The core functionalities for task definition, queuing via Redis, concurrent execution via a worker pool, and an initial event system are now in place.

This version is suitable for further development, testing, and integration of more advanced features like a UI and enhanced task management capabilities.

## Core Features (Current)

* **Task Definition:** Supports defining tasks, with a primary focus on HTTP calls via `ApiTaskPayload`.
* **Redis-backed Queue:** Utilizes Redis lists for robust task queuing, managed by the `pkg/redisq` package.
* **Concurrent Task Execution:** A flexible worker pool (`pkg/workerpool`) manages concurrent goroutines to process tasks efficiently.
* **Pluggable Execution Logic:** Uses a `Task` interface (`pkg/task`) allowing different types of tasks to be executed. An `ApiTaskExecutor` (`pkg/executor`) is provided for HTTP tasks.
* **Event System:** An initial event-driven architecture (`pkg/events`) is implemented, emitting events for key stages in a task's lifecycle (e.g., dequeued, started, succeeded, failed). This will be foundational for UI and monitoring.

## Project Structure & Packages

The project is organized into several key packages under `pkg/`:

* **`pkg/task`**:
    * Defines the `Task` interface that all task executors must implement.
    * Contains `ApiTaskPayload` for defining HTTP tasks and `ApiTaskResult` for their outcomes.
* **`pkg/redisq`**:
    * Provides the `Client` interface and its Redis-based implementation for enqueuing and dequeuing tasks (`ApiTaskPayload` objects) from Redis lists.
* **`pkg/executor`**:
    * Home to concrete implementations of the `task.Task` interface.
    * Currently includes `ApiTaskExecutor` for making HTTP requests based on `ApiTaskPayload`.
* **`pkg/workerpool`**:
    * The core concurrency engine. It manages a pool of worker goroutines.
    * Receives tasks and delegates their execution to a configured `task.Task` implementer.
    * Integrated with the `pkg/events` system to publish task lifecycle events.
* **`pkg/events`**:
    * Defines event types (e.g., `TaskEvent`, `EventType`) and provides an `EventManager` for publishing and subscribing to system events.
* **`cmd/workerapp`**:
    * Contains the main application (`main.go`) that initializes and runs the GoPherQ worker service. It integrates all the `pkg` components.

## Running GoPherQ (Current Development State)

Follow these steps to run the GoPherQ worker application:

**1. Prerequisites:**

* **Go:** Version 1.18 or higher installed.
* **Redis:** A running Redis server instance.

**2. Configuration:**

The worker application (`cmd/workerapp/main.go`) can be configured via environment variables:

* `REDIS_ADDRESS`: Address of your Redis server (default: `localhost:6379`).
* `REDIS_PASSWORD`: Password for your Redis server (default: empty).
* `REDIS_DB`: Redis database number (default: `0`).
* `QUEUE_NAME`: The name of the Redis list to use as a task queue (default: `my_tasks_queue`).
* `NUM_WORKERS`: The number of concurrent worker goroutines (default: `5`).

**3. Build and Run:**

Navigate to the root directory of the GoPherQ project.

* **To run directly:**
    ```bash
    # You can set environment variables inline if needed:
    # QUEUE_NAME="gopherq_tasks" NUM_WORKERS=3 go run ./cmd/workerapp/main.go

    go run ./cmd/workerapp/main.go
    ```
* **To build an executable first:**
    ```bash
    go build -o GoPherQ_worker ./cmd/workerapp/main.go
    ./GoPherQ_worker
    ```

The application will start, connect to Redis, and begin listening for tasks on the configured queue. You will also see event logs for task lifecycle events.

**4. Enqueueing a Test Task (Manual):**

You can manually enqueue a task using `redis-cli`. The task should be a JSON representation of the `task.ApiTaskPayload` struct.

* Connect to Redis: `redis-cli`
* Push a task to the queue (assuming `RPUSH` is used by your `redisq.Enqueue` and your queue is `my_tasks_queue`):
    ```redis
    RPUSH my_tasks_queue '{"id":"task001","method":"GET","url":"[https://jsonplaceholder.typicode.com/todos/1](https://jsonplaceholder.typicode.com/todos/1)","header":{"X-Custom-Header":"GoPherQTest"},"body":""}'
    ```

Observe the logs from your running `GoPherQ_worker` application to see the task being dequeued, processed, and events being logged.

## Roadmap Highlights (Towards v1.0)

* **User Interface (UI):** Develop a web-based UI to monitor queues, tasks, and workers, utilizing the event system.
* **Enhanced Task Functionalities:**
    * Configurable retries for failed tasks.
    * Dead-Letter Queue (DLQ) mechanism for persistently failing tasks.
    * More robust delayed task execution (e.g., using Redis Sorted Sets).
* **Full Library Packaging:** Refine APIs in `pkg/*` for easy use as standalone Go libraries.
* **Comprehensive Documentation:** Detailed API docs, user guides, and examples.
* **Improved Configuration:** Support for configuration files.
* **Deployment Options:** Provide guidance or tools for easier deployment (e.g., Docker images).

## Contributing

(Details to be added if the project becomes open source - e.g., contribution guidelines, how to report issues, etc.)

---
