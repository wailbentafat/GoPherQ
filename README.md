# A Go Worker Pool Implementation

[![Maintainability](https://api.codeclimate.com/v1/badges/3d3c7a37b1590458f2bf/maintainability)](https://codeclimate.com/github/ebarti/go-worker-pool/maintainability) [![Test Coverage](https://api.codeclimate.com/v1/badges/3d3c7a37b1590458f2bf/test_coverage)](https://codeclimate.com/github/ebarti/go-worker-pool/test_coverage)
## Install
```shell
go get github.com/ebarti/go-worker-pool
```

## Usage:

### Simple usage
Instantiate new worker pools via the `NewWorkerPool` factory method.

```go
type MyTask struct {
	c int32
}

func NewTask() *MyTask {
	return &MyTask{}
}

// Add a Run function with the appropriate signature to implement the Task interface 
func (me *MyTask) Run(wg WorkerPool, in interface{}) error {
	atomic.AddInt32(me.c, 1)
}

func main() {
	myTask := NewTask()
    workerPool := NewWorkerPool(context.Background(), myTask, 10).CancelOnSignal(os.Interrupt).Work()
    
    for i := 0; i < 10; i++ {
        workerpool.Send(i)
    }
    
    // Close the workerpool to wait for its completion
    if err := workerPool.Close(); err != nil {
        fmt.Println(err)
        return
    }
    
    fmt.Println()
}



```

### Complex Usage

```go

type type1 string
type type2 string

type MyTask0 struct {}

type MyTask1 struct {
	store []type1
	mu *sync.RWMutex
}

type MyTask2 struct {
	store []type2
	mu *sync.RWMutex
}

func NewTask0() *MyTask0 {
	return &MyTask0{}
}

func NewTask1() *MyTask1 {
	return &MyTask1{mu: new(sync.RWMutex)}
}

func NewTask2() *MyTask2 {
	return &MyTask2{mu: new(sync.RWMutex)}
}

func (my *MyTask0) Run(wg WorkerPool, in interface{}) error {
    i, ok := in.(int)
    if !ok {
        return errors.New("MyTask0 invalid input type")
    }
    outtype1 := type1(strconv.Itoa(i) + " type1")
    outtype2 := type2(strconv.Itoa(i) + " type2")
    wg.Out(outtype1)
    wg.Out(outtype2)
    return nil
}

func (my *MyTask1) Run(wg WorkerPool, in interface{}) error {
    i, ok := in.(type1)
    if !ok {
        return errors.New("MyTask1 invalid input type")
    }
    my.mu.Lock()
    my.store = append(my.store, i)
    my.mu.Unlock()
    return nil
}


func (my *MyTask2) Run(wg WorkerPool, in interface{}) error {
    i, ok := in.(type2)
    if !ok {
        return errors.New("MyTask2 invalid input type")
    }

    my.mu.Lock()
    my.store = append(my.store, i)
    my.mu.Unlock()
    return nil
}

func main() {
    ctx := context.Background()
    t0 := NewTask0()
    t1 := NewTask1()
    t2 := NewTask2()
    worker0 := NewWorkerPool(ctx, NewTask0(), 20).Work()
    worker1 := NewWorkerPool(ctx, t1, 20).ReceiveFromWithType(reflect.TypeOf(t1), worker0).Work()
    worker2 := NewWorkerPool(ctx, t2, 20).ReceiveFromWithType(reflect.TypeOf(t1), worker0).Work()
    for i := 0; i < 2000; i++ {
        workerOne.Send(i)
    }
    if err := worker0.Close(); err != nil {
        fmt.Println(err)
    }
    if err := worker1.Close(); err != nil {
        fmt.Println(err)
    }
    if err := worker2.Close(); err != nil {
        fmt.Println(err)
    }
}

```


## Version log

- v0.1.0 Initial release
