package workerpool

import (
	"context"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/wailbentafat/go-worker-pool/pkg/task"
)



const (
	DefaultWorkerCount=5
	DefaultQueueName="default_tasks_queue"
	DefaultQueueTimeout=5 * 60 
	DefaultQueueDelay=0 * time.Second
	sigChanBufferSize=1
)
type Workerpool interface{
 Send(interface{})
 RecieveFrom(t reflect.Type,inworker ...Workerpool)
 Work() Workerpool
 OutChannel(t reflect.Type,out chan interface{} )
 CancelOnsignal(signals ...os.Signal) Workerpool
 Close() error


}
type workerpool struct {
	Ctx context.Context
	workerTask  task.Task
	err error
	numberOfworker int64
	inChan chan interface{}
	internaloutChan chan interface{}
	outtypedchan map[reflect.Type]chan interface{}
	sigChan  chan os.Signal
	cancel context.CancelFunc
	semaphore chan struct{}
	isLeader bool
	wg *sync.WaitGroup
	onceErr *sync.Once
	onceCloseOut *sync.Once
}