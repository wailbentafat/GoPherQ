package executor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/wailbentafat/go-worker-pool/pkg/task"
)

type ApiTaskExecutor struct {
	HttpClient *http.Client
}

func NewApiTaskExecutor(client *http.Client) *ApiTaskExecutor {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &ApiTaskExecutor{
		HttpClient: client,
	}
}

// Run implements the task.Task interface.
// 'in' is expected to be of type task.ApiTaskPayload.
// 'out' is an optional channel to send back a task.ApiTaskResult.
func (ate *ApiTaskExecutor) Run(in interface{}, out chan<- interface{}) error {
	// 1. Type Assertion: Ensure 'in' is the expected task.ApiTaskPayload
	payload, ok := in.(task.ApiTaskPayload)
	if !ok {
		err := fmt.Errorf("executor: received unexpected task type. Expected task.ApiTaskPayload, got %T", in)
		if out != nil {
			taskID := "unknown"
			if p, ok := in.(interface{ GetID() string }); ok {
				taskID = p.GetID()
			} else if p, okConv := in.(task.ApiTaskPayload); okConv {
				taskID = p.Id
			}

			select {
			case out <- task.ApiTaskResult{TaskID: taskID, Error: err.Error()}:
			default:
				//add retry logic here TODO:
			}
		}
		return err
	}

	startTime := time.Now()
	fmt.Printf("Executor bda task ID %s: %s %s\n", payload.Id, payload.Method, payload.Url)

	var reqBodyReader *bytes.Reader
	if payload.Body != "" {
		reqBodyReader = bytes.NewReader([]byte(payload.Body))
	} else {
		reqBodyReader = bytes.NewReader([]byte{})
	}

	reqCtx, cancelReqCtx := context.WithTimeout(context.Background(), ate.HttpClient.Timeout)
	defer cancelReqCtx()

	req, err := http.NewRequestWithContext(reqCtx, payload.Method, payload.Url, reqBodyReader)
	if err != nil {
		err = fmt.Errorf("executor: failed to create HTTP request for task %s: %w", payload.Id, err)
		if out != nil {
			select {
			case out <- task.ApiTaskResult{TaskID: payload.Id, Error: err.Error(), Duration: time.Since(startTime)}:
			default:
			}
		}
		return err
	}

	for key, value := range payload.Header {
		req.Header.Set(key, value)
	}
	if payload.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		//add security token hnaya
	}

	resp, err := ate.HttpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("executor: HTTP request execution failed for task %s (%s %s): %w", payload.Id, payload.Method, payload.Url, err)
		if out != nil {
			select {
			case out <- task.ApiTaskResult{TaskID: payload.Id, Error: err.Error(), Duration: time.Since(startTime)}:
			default:
			}
		}
		return err
	}
	defer resp.Body.Close()

	responseBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body for task %s: %w", payload.Id, err)
		if out != nil {
			select {
			case out <- task.ApiTaskResult{TaskID: payload.Id, StatusCode: resp.StatusCode, Error: err.Error(), Duration: time.Since(startTime)}:
			default:
			}
		}
		return err
	}
	responseBodyString := string(responseBodyBytes)
	duration := time.Since(startTime)

	fmt.Printf("Executor: Task ID %s finished in %v. Status: %d\n", payload.Id, duration, resp.StatusCode)

	taskResult := task.ApiTaskResult{
		TaskID:     payload.Id,
		StatusCode: resp.StatusCode,
		Response:   responseBodyString,
		Duration:   duration,
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		taskResult.Error = fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	}

	if out != nil {
		select {
		case out <- taskResult:
		default:
			fmt.Printf("Executor: Warning - could not send result for task %s to output channel.\n", payload.Id)
		}
	}

	// if resp.StatusCode < 200 || resp.StatusCode >= 300 {
	// 	return fmt.Errorf("HTTP error for task %s: status code %d", payload.Id, resp.StatusCode)
	// }

	return nil
}
