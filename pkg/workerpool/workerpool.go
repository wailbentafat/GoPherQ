package workerpool

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"time"

	"github.com/wailbentafat/go-worker-pool/pkg/task"
)

const (
	DefaultWorkerCount  = 5
	DefaultQueueName    = "default_tasks_queue"
	DefaultQueueTimeout = 5 * 60
	DefaultQueueDelay   = 0 * time.Second
	sigChanBufferSize   = 1
)

type Workerpool interface {
	Send(interface{})
	RecieveFrom(t reflect.Type, inworker ...Workerpool) Workerpool
	Work() Workerpool
	OutChannel(t reflect.Type, out chan interface{})
	CancelOnsignal(signals ...os.Signal) Workerpool
	Close() error
}
type workerpool struct {
	Ctx             context.Context
	workerTask      task.Task
	err             error
	numberOfworker  int64
	inChan          chan interface{}
	internaloutChan chan interface{}
	outtypedchan    map[reflect.Type][]chan interface{}
	sigChan         chan os.Signal
	cancel          context.CancelFunc
	semaphore       chan struct{}
	isLeader        bool
	wg              *sync.WaitGroup
	onceErr         *sync.Once
	onceCloseOut    *sync.Once
}

func (wp *workerpool) RecieveFrom(t reflect.Type, inworker ...Workerpool) Workerpool {
	wp.isLeader = false
	for _, worker := range inworker {
		worker.OutChannel(t, wp.inChan)
	}
	return wp
}
func (wp *workerpool) out(outputData interface{}) error {
	outputType := reflect.TypeOf(outputData)
	if outputType == nil {
	}

	selectedChans, specificTypeExists := wp.outtypedchan[outputType]

	if !specificTypeExists || len(selectedChans) == 0 {
		return nil
	}

	select {
	case <-wp.Ctx.Done():
		return wp.Ctx.Err()
	default:
		teevalue(outputData, selectedChans...)
		return nil
	}
}

func (wp *workerpool) Send(in interface{}) {
	select {
	case <-wp.Ctx.Done():
		log.Printf("Worker pool  context canceled mkch tasks wkhdokrin"+
			" cannot send task: %v", in)
		return
	default:
	}
	select {
	case <-wp.Ctx.Done():
		log.Printf("Worker pool context canceled, cannot send task: %v", in)
		return
	case wp.inChan <- in:
		log.Printf("Task sent to worker pool: %v", in)
	}
}
func (wp *workerpool) runOutChanMux() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.Ctx.Done():
			log.Println("OutChanMux shutting down due to context cancellation")
			for range wp.internaloutChan {

			}
			return
		case outdata, ok := <-wp.internaloutChan:
			if !ok {
				log.Println("OutChanMux shutting down due to context cancellation")
				return
			}
			if err := wp.out(outdata); err != nil {
				wp.onceErr.Do(func() {
					wp.err = err
					log.Println("hbss context  wla err f chun out max ")
				})

			}

		}
	}
}
func (wp *workerpool) ReceiveFrom(t reflect.Type, inWorker ...Workerpool) Workerpool {
	wp.isLeader = false
	for _, worker := range inWorker {
		worker.OutChannel(t, wp.inChan)
	}
	return wp

}

func (wp *workerpool) Work() Workerpool {
	wp.wg.Add(1)
	go wp.runOutChanMux()
	// wp.wg.Add(int(wp.numberOfworker))
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		var taskWg = new(sync.WaitGroup)
		for i := int64(0); i < wp.numberOfworker; i++ {
			wp.semaphore <- struct{}{}
			taskWg.Add(1)
			go func(workerId int64) {
				defer func() {
					<-wp.semaphore
					taskWg.Done()
				}()
				for {
					select {
					case <-wp.Ctx.Done():
						log.Printf("Worker pool context canceled, worker %d exiting", workerId)
						return
					case Taskinput, ok := <-wp.inChan:
						if !ok {
							log.Printf("Worker pool input channel closed, worker %d exiting", workerId)
							return
						}
						log.Printf("Worker %d received task: %v", workerId, Taskinput)
						if err := wp.workerTask.Run(Taskinput, wp.internaloutChan); err != nil {
							wp.onceErr.Do(func() {
								wp.err = err
								log.Printf("Error executing task in worker %d: %v", workerId, err)
								wp.cancel()
							})
						}
						log.Printf("Worker %d finished task: %v", workerId, Taskinput)

					}
				}
			}(i)
		}
		taskWg.Wait()
		log.Println("All workers have finished their tasks, closing worker pool")

	}()
	return wp
}

func (wp *workerpool) OutChannel(t reflect.Type, out chan interface{}) {
	if _, ok := wp.outtypedchan[t]; !ok {
		wp.outtypedchan[t] = []chan interface{}{out}
	} else {
		wp.outtypedchan[t] = append(wp.outtypedchan[t], out)
	}
}

func (wp *workerpool) Close() error {
	log.Printf("initiiling the closing ")
	wp.cancel()
	wp.closeChannels()
	if wp.Ctx.Err() != nil && !errors.Is(wp.Ctx.Err(), context.Canceled) && !errors.Is(wp.Ctx.Err(), context.DeadlineExceeded) {
		if wp.err == nil {
			return wp.Ctx.Err()
		}
		return nil
	}
	return wp.err
}

func (wp *workerpool) closeChannels() {
	wp.onceCloseOut.Do(func() {
		if wp.inChan != nil {
			close(wp.inChan)
		}
		if wp.internaloutChan != nil {
			close(wp.internaloutChan)
		}
	})
	wp.wg.Wait()
}
func (wp *workerpool) waitForSignal(signale ...os.Signal) {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		select {
		case s := <-wp.sigChan:
			log.Printf("raj ja signal %v cancel context dok ", s)
		case <-wp.Ctx.Done():
			log.Printf("context canceled")
		}
	}()
	signal.Notify(wp.sigChan, signale...)
}

func (wp *workerpool) CancelOnsignal(signals ...os.Signal) Workerpool {
	if len(signals) > 0 {
		wp.waitForSignal(signals...)
	}
	return wp
}
func teevalue(value interface{}, destination ...chan interface{}) {
	for _, dest := range destination {
		select {
		case dest <- value:
		default:
			log.Println("mrhich tb3t l destination channel ray habssa")
		}
	}

}
func NewWorkerPool(ctx context.Context, workerexcuter task.Task, numberofworker int64) Workerpool {
	if numberofworker <= 0 {
		numberofworker = DefaultWorkerCount
	}
	c, cancel := context.WithCancel(ctx)
	return &workerpool{

		Ctx:             c,
		workerTask:      workerexcuter,
		numberOfworker:  numberofworker,
		inChan:          make(chan interface{}, 100),
		internaloutChan: make(chan interface{}, 100),
		outtypedchan:    make(map[reflect.Type][]chan interface{}),
		sigChan:         make(chan os.Signal, sigChanBufferSize),
		cancel:          cancel,
		semaphore:       make(chan struct{}, numberofworker),
		isLeader:        true,
		wg:              &sync.WaitGroup{},
		onceErr:         &sync.Once{},
		onceCloseOut:    &sync.Once{},
	}
}
