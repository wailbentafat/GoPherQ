package events


import (
	"time"
	// "github.com/wailbentafat/GoPherQ/pkg/task"
)

type EventType string

const (
	EventTypeTaskDequeued  EventType = "TASK_DEQUEUED" 
	EventTypeTaskStarted   EventType = "TASK_STARTED"  
	EventTypeTaskSucceeded EventType = "TASK_SUCCEEDED" 
	EventTypeTaskFailed    EventType = "TASK_FAILED"    
)

type TaskEvent struct {
	Timestamp time.Time
	Type      EventType
	TaskID    string    
	WorkerID  string    
	QueueName string   
	Error     string   
	Details   string   
}

func NewTaskEvent(eventType EventType, taskID string) TaskEvent {
	return TaskEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		TaskID:    taskID,
	}
}package events

import (
	"fmt" // For optional logging in EventManager, can be removed if not used
	"sync"
	"time"
	// You might need to import your task package if events need to contain
	// full task details directly, rather than just IDs or summaries.
	// For example:
	// "github.com/wailbentafat/GoPherQ/pkg/task"
)

// EventType defines the type of an event.
type EventType string

// Constants for different event types related to tasks.
const (
	EventTypeTaskEnqueued  EventType = "TASK_ENQUEUED"  // When a task is first added to the system (e.g., by a producer)
	EventTypeTaskDequeued  EventType = "TASK_DEQUEUED"  // When a worker picks it from the queue
	EventTypeTaskStarted   EventType = "TASK_STARTED"   // When an executor begins Run() for a task
	EventTypeTaskSucceeded EventType = "TASK_SUCCEEDED" // When a task completes successfully via its executor
	EventTypeTaskFailed    EventType = "TASK_FAILED"    // When a task fails during execution via its executor
	// Future event types could include:
	// EventTypeWorkerStarted EventType = "WORKER_STARTED"
	// EventTypeWorkerStopped EventType = "WORKER_STOPPED"
	// EventTypeQueueStats    EventType = "QUEUE_STATS"
)

// TaskEvent holds information about an event related to a task's lifecycle.
type TaskEvent struct {
	Timestamp time.Time // Time the event occurred
	Type      EventType // Type of the event (e.g., TASK_STARTED)
	TaskID    string    // The ID of the task this event pertains to
	WorkerID  string    // Optional: Identifier of the worker that processed/is processing the task
	QueueName string    // Optional: Name of the queue the task originated from
	Error     string    // Optional: Error message, typically used with EventTypeTaskFailed
	Details   string    // Optional: Any other relevant details or a summary of the event or task result
	// Payload   interface{} // Optional: Could include the task.ApiTaskPayload; consider size and necessity
	// Result    interface{} // Optional: Could include the task.ApiTaskResult; consider size and necessity
}

// NewTaskEvent is a helper function to create and initialize a new TaskEvent.
func NewTaskEvent(eventType EventType, taskID string) TaskEvent {
	return TaskEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		TaskID:    taskID,
	}
}

// EventManager handles publishing and subscribing to TaskEvents.
// This is a simple in-memory broadcaster. For distributed systems or very high
// event volumes, an external message bus (like Redis Pub/Sub, Kafka, NATS)
// might be more appropriate.
type EventManager struct {
	subscribers []chan TaskEvent 
	mu          sync.RWMutex     
	isClosed    bool             
	closedChan  chan struct{}    
}

func NewEventManager() *EventManager {
	return &EventManager{
		subscribers: make([]chan TaskEvent, 0),
		closedChan:  make(chan struct{}),
	}
}
func (em *EventManager) Publish(event TaskEvent) {
	em.mu.RLock() 
	if em.isClosed {
		em.mu.RUnlock()
		return
	}
	activeSubscribers := make([]chan TaskEvent, len(em.subscribers))
	copy(activeSubscribers, em.subscribers)
	em.mu.RUnlock() 

	for _, subChan := range activeSubscribers {
		select {
		case subChan <- event:
		default:
			fmt.Printf("EventManager: Warning - event dropped for a subscriber (channel full/closed). TaskID: %s, Type: %s\n", event.TaskID, event.Type)
		}
	}
}
func (em *EventManager) Subscribe() <-chan TaskEvent {
	em.mu.Lock() 
	defer em.mu.Unlock()

	if em.isClosed {
		closedCh := make(chan TaskEvent)
		close(closedCh)
		return closedCh
	}
	subChan := make(chan TaskEvent, 100) 
	em.subscribers = append(em.subscribers, subChan)
	go func(ch chan TaskEvent) {
		<-em.closedChan 

		close(ch)
	}(subChan)

	return subChan 
}

func (em *EventManager) Close() {
	em.mu.Lock()
	defer em.mu.Unlock()

	if !em.isClosed {
		em.isClosed = true
		close(em.closedChan)
		em.subscribers = nil
		fmt.Println("EventManager: Closed.")
	}
}