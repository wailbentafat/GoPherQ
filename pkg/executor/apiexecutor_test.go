package executor

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/wailbentafat/go-worker-pool/pkg/task"
)


type ApitaskExecuter  struct{
	Client *http.Client
}
func NewApiTaskExecuter(client *http.Client) *ApitaskExecuter {
	return &ApitaskExecuter{
		Client: &http.Client{
			Timeout: 10 * 1000, 
		},
	}
}
func (e *ApitaskExecuter)Run (in interface{}, out chan<- interface{}) {
	TaskDetail,ok:=in.(task.ApiTaskPayload)
	if !ok {
		out <- task.ApiTaskResult{
			TaskID: TaskDetail.Id,
			Error: "Invalid input type, expected ApiTaskPayload",
		}
	}
	var reqBodyReader *strings.Reader
	if TaskDetail.Body != "" {
		reqBodyReader = strings.NewReader(TaskDetail.Body)
	} else {
		reqBodyReader = nil
	}
	req,err:= http.NewRequest(TaskDetail.Method, TaskDetail.Url, reqBodyReader)
	if err != nil {
		out <- task.ApiTaskResult{
			TaskID: TaskDetail.Id,
			Error:  "Failed to create HTTP request: " + err.Error(),
		}
		return
	}
	fmt.Printf("Executing API task: %s %s\n ", TaskDetail.Method, TaskDetail.Url)
	fmt.Printf("Request Headers: %v\n", req)

	
}
