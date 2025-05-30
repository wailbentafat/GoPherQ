package task

import "time"

type ApiTaskPayload struct{
	Id string `json:"id"`
	Header map[string]string `json:"header"`
	Body string `json:"body"`
	Method string `json:"method"`
	Url string `json:"url"`
}
type ApiTaskResult struct {
    TaskID     string
    StatusCode int
    Response   string 
    Error      string
    Duration   time.Duration
}
type Task interface{
	Run (in interface{}, out chan<- interface{}) error
}