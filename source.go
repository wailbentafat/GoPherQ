// package main



// import (



// "context"



// "errors"

// "fmt"


// "log"

// "net/http"

// "os"

// "os/signal"

// "reflect"

// "strings"

// "sync"

// "time"





// )



// const (

// sigChanBufferSize = 1

// redisScheduledTasksKey = "scheduled_api_tasks"

// redisProcessingTasksKey= "processing_api_tasks" // For more robust locking

// )



// // --- WorkerPool Interface and Task Interface (from original prompt) ---



// type WorkerPool interface {

// Send(interface{})

// ReceiveFrom(t reflect.Type, inWorker ...WorkerPool) WorkerPool

// Work() WorkerPool

// OutChannel(t reflect.Type, out chan interface{})

// CancelOnSignal(signals ...os.Signal) WorkerPool

// Close() error

// }



// // Task : interface to be implemented by a desired type

// // Its Run method is called by workerPool onceErr the workers start

// // and there is data sent to the channel via its Send method

// type Task interface {

// Run(in interface{}, out chan<- interface{}) error

// }



// // --- ApiTaskDetails and ApiTaskResult structs ---



// // ApiTaskDetails holds the information needed to make an HTTP request.

// type ApiTaskDetails struct {

// ID string `json:"id"` // Unique identifier for the task

// URL string `json:"url"` // URL to call

// Method string `json:"method"` // HTTP method (GET, POST, etc.)

// Headers map[string]string `json:"headers,omitempty"` // Optional HTTP headers

// Body string `json:"body,omitempty"` // Optional request body (as string)

// }



// // ApiTaskResult structure for results from API calls (optional usage)

// type ApiTaskResult struct {

// TaskID string

// StatusCode int

// Response string

// Error string

// Duration time.Duration

// }



// // --- Stub for utils.TeeValue ---

// // As github.com/ebarti/utils is external, providing a basic stub.

// // A production version might have more features (e.g., non-blocking, context-aware).

// func teeValue(value interface{}, destinations ...chan interface{}) {

// for _, dest := range destinations {

// // Basic send; could block if channel is full.

// // A more robust version might use a select with a timeout or context.

// select {

// case dest <- value:

// default:

// log.Printf("TeeValue: Failed to send to a destination channel (possibly full or closed).")

// }

// }

// }



// // --- WorkerPool Implementation (workerPool struct and its methods) ---



// // workerPool : a pool of workers that asynchronously execute a given task

// type workerPool struct {

// Ctx context.Context

// workerTask Task

// err error

// numberOfWorkers int64

// inChan chan interface{}

// internalOutChan chan interface{}

// outTypedChan map[reflect.Type][]chan interface{}

// sigChan chan os.Signal

// cancel context.CancelFunc

// semaphore chan struct{}

// isLeader bool

// wg *sync.WaitGroup

// onceErr *sync.Once

// onceCloseOut *sync.Once

// }



// // NewWorkerPool : workerPool factory. Needs a defined number of workers to instantiate.

// func NewWorkerPool(ctx context.Context, workerFunction Task, numberOfWorkers int64) WorkerPool {

// c, cancel := context.WithCancel(ctx)

// return &workerPool{

// numberOfWorkers: numberOfWorkers,

// Ctx: c,

// workerTask: workerFunction,

// inChan: make(chan interface{}, numberOfWorkers),

// internalOutChan: make(chan interface{}, numberOfWorkers),

// sigChan: make(chan os.Signal, sigChanBufferSize),

// outTypedChan: make(map[reflect.Type][]chan interface{}),

// cancel: cancel,

// semaphore: make(chan struct{}, numberOfWorkers),

// isLeader: true,

// wg: new(sync.WaitGroup),

// onceErr: new(sync.Once),

// onceCloseOut: new(sync.Once),

// }

// }



// // Send : send to workers in channel

// func (wp *workerPool) Send(in interface{}) {

// // Check context before attempting send to prevent blocking if pool is closing/closed

// select {

// case <-wp.Ctx.Done():

// log.Printf("WorkerPool: Context cancelled, not sending task: %+v\n", in)

// return

// default:

// // Non-blocking send to inChan to avoid deadlocking if Send is called after Close

// // or if inChan is full and workers are slow/stuck.

// // However, the original design blocks on send if inChan is full.

// // Reverting to original blocking send, relying on context for cancellation.

// }



// select {

// case <-wp.Ctx.Done():

// log.Printf("WorkerPool: Context cancelled during send, task not sent: %+v\n", in)

// return

// case wp.inChan <- in:

// return

// }

// }



// // ReceiveFrom : assigns workers out channel to this workers in channel

// func (wp *workerPool) ReceiveFrom(t reflect.Type, inWorker ...WorkerPool) WorkerPool {

// wp.isLeader = false

// for _, worker := range inWorker {

// worker.OutChannel(t, wp.inChan)

// }

// return wp

// }



// // OutChannel : Sets the workers output channel to one provided.

// func (wp *workerPool) OutChannel(t reflect.Type, out chan interface{}) {

// if _, ok := wp.outTypedChan[t]; !ok {

// var outs []chan interface{}

// outs = append(outs, out)

// wp.outTypedChan[t] = outs

// } else {

// wp.outTypedChan[t] = append(wp.outTypedChan[t], out)

// }

// }



// // Work : starts workers

// func (wp *workerPool) Work() WorkerPool {

// wp.wg.Add(1) // For the runOutChanMux goroutine



// // Start the output multiplexer

// go wp.runOutChanMux()



// wp.wg.Add(1) // For the main worker dispatching goroutine

// go func() {

// defer wp.wg.Done()

// var taskWg = new(sync.WaitGroup) // WaitGroup for individual tasks spawned



// for {

// select {

// case <-wp.Ctx.Done():

// // Context cancelled, stop processing new tasks from inChan.

// // Wait for any tasks already dispatched by this loop to complete.

// taskWg.Wait()

// // Drain inChan to allow senders to not block indefinitely if they don't check context.

// // This part is tricky; if inChan is closed by Close(), this loop also exits.

// // If Ctx is Done, but inChan is not yet closed, ranging over it would still work

// // until it's empty or closed.

// log.Println("WorkerPool: Work dispatcher shutting down due to context cancellation.")

// return

// case in, ok := <-wp.inChan:

// if !ok {

// // inChan is closed, means no more tasks will come.

// // Wait for tasks dispatched by this loop to complete.

// taskWg.Wait()

// log.Println("WorkerPool: inChan closed, work dispatcher shutting down.")

// return

// }



// // Acquire semaphore before launching goroutine

// select {

// case wp.semaphore <- struct{}{}:

// taskWg.Add(1)

// go func(taskInput interface{}) {

// defer func() {

// <-wp.semaphore

// taskWg.Done()

// }()



// // Check context again before running the task, in case it was cancelled

// // while waiting for the semaphore.

// select {

// case <-wp.Ctx.Done():

// log.Printf("WorkerPool: Context cancelled before task execution: %+v\n", taskInput)

// // Set error if this is the first cancellation notice for the pool

// wp.onceErr.Do(func() {

// wp.err = context.Canceled

// // wp.cancel() // Already cancelled, or will be by main.

// })

// return

// default:

// }


// if err := wp.workerTask.Run(taskInput, wp.internalOutChan); err != nil {

// wp.onceErr.Do(func() {

// wp.err = err

// log.Printf("WorkerPool: Task error, cancelling context: %v\n", err)

// wp.cancel() // Cancel context on first task error

// })

// return // Don't proceed if task run failed

// }

// }(in)

// case <-wp.Ctx.Done():

// // Context cancelled while waiting to acquire semaphore.

// // The task `in` is effectively dropped.

// log.Printf("WorkerPool: Context cancelled while acquiring semaphore for task: %+v\n", in)

// // No need to call taskWg.Done() as we didn't Add for this task.

// // The outer loop will select Ctx.Done() next or inChan close.

// }

// }

// }

// }()

// return wp

// }



// // out : pushes value to workers out channel

// // Used when chaining worker pools.

// func (wp *workerPool) out(outputData interface{}) error {

// // Determine the type of the output data to find matching channels

// outputType := reflect.TypeOf(outputData)

// if outputType == nil { // Handle case where outputData is a nil interface

// outputType = reflect.TypeOf((*interface{})(nil)).Elem()

// }





// selectedChans, specificTypeExists := wp.outTypedChan[outputType]



// // If no channels for the specific type, check for "any type" (nil key) channels

// if !specificTypeExists {

// anyTypeChans, anyTypeExists := wp.outTypedChan[nil] // or a predefined key for "any"

// if anyTypeExists {

// selectedChans = append(selectedChans, anyTypeChans...)

// }

// }


// if len(selectedChans) == 0 {

// // log.Printf("WorkerPool: Couldn't locate out chan for type %v or any-type chan.\n", outputType)

// return nil // Not an error to have no output channels, just means output is dropped.

// // return errors.New("couldn't locate out chan for type " + outputType.String())

// }



// select {

// case <-wp.Ctx.Done():

// return context.Canceled // Or return nil if just stopping

// default:

// teeValue(outputData, selectedChans...) // Use the stubbed/actual TeeValue

// return nil

// }

// }



// // runOutChanMux : multiplex the out channel to the right destination

// func (wp *workerPool) runOutChanMux() {

// defer wp.wg.Done()

// for {

// select {

// case <-wp.Ctx.Done():

// log.Println("WorkerPool: OutChanMux shutting down due to context cancellation.")

// // Drain internalOutChan? If tasks are still producing output, this might be useful.

// // However, if Ctx is done, tasks should also be stopping.

// for range wp.internalOutChan {

// // Discard remaining items

// }

// return

// case out, ok := <-wp.internalOutChan:

// if !ok {

// log.Println("WorkerPool: internalOutChan closed, OutChanMux shutting down.")

// return // Channel closed

// }

// if err := wp.out(out); err != nil {

// wp.onceErr.Do(func() {

// wp.err = err

// log.Printf("WorkerPool: Error in outChanMux, cancelling context: %v\n", err)

// wp.cancel() // Cancel context on output error

// })

// // If context is cancelled, this loop will exit on the next iteration's Ctx.Done()

// }

// }

// }

// }



// // waitForSignal : make sure we wait for a term signal and shutdown correctly

// func (wp *workerPool) waitForSignal(signals ...os.Signal) {

// wp.wg.Add(1) // Add to WaitGroup for this goroutine

// go func() {

// defer wp.wg.Done() // Ensure Done is called when goroutine exits

// select {

// case s := <-wp.sigChan:

// log.Printf("WorkerPool: Received signal: %v. Cancelling context.\n", s)

// if wp.cancel != nil {

// wp.cancel()

// }

// case <-wp.Ctx.Done():

// // Context was cancelled by other means (e.g. error, external cancel)

// log.Println("WorkerPool: waitForSignal goroutine exiting due to context cancellation.")

// }

// }()

// signal.Notify(wp.sigChan, signals...)

// }



// // CancelOnSignal : set the signal to be used to cancel the pool

// func (wp *workerPool) CancelOnSignal(signals ...os.Signal) WorkerPool {

// if len(signals) > 0 {

// wp.waitForSignal(signals...)

// }

// return wp

// }



// // closeChannels : closes channels and waits for main worker goroutines

// func (wp *workerPool) closeChannels() error {

// wp.onceCloseOut.Do(func() {

// // Close inChan first. This signals the main dispatcher goroutine in Work() to stop accepting new tasks.

// if wp.inChan != nil {

// close(wp.inChan)

// }

// // Then close internalOutChan. This signals runOutChanMux to stop.

// if wp.internalOutChan != nil {

// close(wp.internalOutChan)

// }

// // Close all registered external output channels

// // This signals consumers that no more data will come from this pool.

// // Caution: Closing channels not owned by this component can be risky if they are shared.

// // However, in this design, OutChannel implies the pool might take some ownership for signaling 'done'.

// // For simplicity, we won't close external channels here to avoid panics if they are closed elsewhere.

// // Consumers should ideally react to the pool's context cancellation or rely on the specific

// // output channels being closed by their direct owners if not the pool.

// })



// // Wait for the main goroutines (Work dispatcher, runOutChanMux, waitForSignal) to complete.

// // These goroutines are responsible for managing their own spawned goroutines (e.g., individual tasks).

// wp.wg.Wait()

// return wp.err

// }



// // Close : gracefully shuts down the worker pool.

// func (wp *workerPool) Close() error {

// log.Println("WorkerPool: Close called. Initiating shutdown...")



// // 1. Signal cancellation if not already done (e.g., if Close is called directly)

// // If cancel was already called (e.g., by signal or error), this is a no-op.

// if wp.cancel != nil {

// wp.cancel()

// }


// // The original blocking loop `for len(wp.inChan) > 0 || len(wp.semaphore) > 0`

// // is problematic because if context is cancelled, workers might not pick items from inChan,

// // and semaphore might not clear if tasks are stuck or context cancelled mid-task.

// // The shutdown relies on context cancellation propagating and channels closing.



// // 2. Close channels and wait for core goroutines to finish.

// // This will wait for the Work() dispatcher and runOutChanMux() to finish.

// // The Work() dispatcher, upon seeing inChan close or Ctx.Done(), waits for its spawned tasks.

// shutdownErr := wp.closeChannels()



// finalErr := wp.err

// if shutdownErr != nil && !errors.Is(shutdownErr, context.Canceled) && !errors.Is(shutdownErr, context.DeadlineExceeded) {

// if finalErr == nil {

// finalErr = shutdownErr

// } else {

// finalErr = fmt.Errorf("pool error: %v, shutdown error: %v", finalErr, shutdownErr)

// }

// }


// if finalErr != nil && !errors.Is(finalErr, context.Canceled) && !errors.Is(finalErr, context.DeadlineExceeded) {

// log.Printf("WorkerPool: Closed with error: %v\n", finalErr)

// return finalErr

// }



// log.Println("WorkerPool: Shutdown complete.")

// return nil

// }



// // --- ApiTaskExecutor Implementation ---



// // ApiTaskExecutor implements Task to execute API requests.

// type ApiTaskExecutor struct{}



// func (ate *ApiTaskExecutor) Run(in interface{}, out chan<- interface{}) error {

// taskDetails, ok := in.(ApiTaskDetails)

// if !ok {

// err := fmt.Errorf("invalid task type received by ApiTaskExecutor. Expected ApiTaskDetails, got %T", in)

// log.Println(err)

// // Optionally send a structured error to the out channel if needed

// // if out != nil { out <- ApiTaskResult{Error: err.Error()} }

// return err

// }



// startTime := time.Now()

// fmt.Print(startTime)

// log.Printf("ApiTaskExecutor: Executing task ID %s: %s %s\n", taskDetails.ID, taskDetails.Method, taskDetails.URL)



// var reqBodyReader *strings.Reader

// if taskDetails.Body != "" {

// reqBodyReader = strings.NewReader(taskDetails.Body)

// } else {

// reqBodyReader = strings.NewReader("") // Empty body

// }



// // Assuming 'in' contains a context, which it doesn't in the current Task interface.

// // For cancellable HTTP requests, the worker pool's context (wp.Ctx) could be passed down

// // or a new context derived from it could be part of the `in` interface if Task.Run was changed.

// // For now, we use a default HTTP client without explicit context for the request itself,

// // relying on worker pool context for starting/stopping the task run.

// req, err := http.NewRequest(taskDetails.Method, taskDetails.URL, reqBodyReader)

// if err != nil {

// log.Printf("ApiTaskExecutor: Error creating request for task %s: %v\n", taskDetails.ID, err)

// // if out != nil { out <- ApiTaskResult{TaskID: taskDetails.ID, Error: err.Error(), Duration: time.Since(startTime)} }

// return fmt.Errorf("creating request for task %s failed: %w", taskDetails.ID, err)

// }



// for key, value := range taskDetails.Headers {

// req.Header.Set(key, value)

// }

// if taskDetails.Body != "" && req.Header.Get("Content-Type") == "" {

// req.Header.Set("Content-Type", "application/json; charset=utf-8")

// }



// client := &http.Client{Timeout: 30 * time.Second}

// resp, err := client.Do(req)

// if err != nil {

// log.Printf("ApiTaskExecutor: Error performing request for task %s: %v\n", taskDetails.ID, err)

// // if out != nil { out <- ApiTaskResult{TaskID: taskDetails.ID, Error: err.Error(), Duration: time.Since(startTime)} }

// return fmt.Errorf("performing request for task %s failed: %w", taskDetails.ID, err)

// }

// defer resp.Body.Close()



// // Optionally send a structured error to the out channel if needed

// // if out != nil { out <- ApiTaskResult{TaskID: taskDetails.ID, StatusCode: resp.StatusCode, Response: resp.Body, Duration: time.Since(startTime)} }

// return nil

// }



package goworkerpool
