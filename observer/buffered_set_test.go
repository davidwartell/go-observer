package observer

import (
	"context"
	"github.com/pkg/errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestClose(t *testing.T) {
	var o = NewBufferedSetObserver(0 * time.Second)
	o.Close()
}

func TestAddListener(t *testing.T) {
	var output string
	var o = NewBufferedSetObserver(0 * time.Second)
	defer o.Close()

	done := make(chan bool)
	defer close(done)

	o.AddListener(func(ctx context.Context, e []string) {
		output = e[0]
		done <- true
	})

	o.Emit("done")

	<-done // blocks until listener is triggered

	if output != "done" {
		t.Error("error Emitting strings.")
	}
}

func TestEmit(t *testing.T) {
	var output string
	var o = NewBufferedSetObserver(0 * time.Second)
	defer o.Close()

	done := make(chan bool)
	defer close(done)

	o.AddListener(func(ctx context.Context, e []string) {
		output = e[0]
		done <- true
	})

	o.Emit("done")

	<-done // blocks until listener is triggered

	if output != "done" {
		t.Error("error Emitting strings.")
	}
}

func TestEmitParallel(t *testing.T) {
	var o = NewBufferedSetObserver(0 * time.Second)
	defer o.Close()

	countExpected := uint64(1000)
	doneChan := make(chan interface{})
	observer := &testObserver{
		countExpected: countExpected,
		doneChan:      doneChan,
	}

	o.AddListener(observer.Observe)

	for i := uint64(0); i < countExpected; i++ {
		num := i
		go func() {
			o.Emit("done " + strconv.FormatUint(num, 10))
		}()
	}

	// blocks until listener is triggered numRoutines times
	<-doneChan
}

func TestEmitParallelBuffered(t *testing.T) {
	bufferDuration := uint64(1)
	var o = NewBufferedSetObserver(time.Duration(bufferDuration) * time.Second)
	defer o.Close()

	countExpected := uint64(1000)
	doneChan := make(chan interface{})
	observer := &testObserver{
		countExpected: countExpected,
		doneChan:      doneChan,
	}

	o.AddListener(observer.Observe)

	sleepMs := ((bufferDuration * uint64(2)) * uint64(1000)) / countExpected
	for i := uint64(0); i < countExpected; i++ {
		num := i
		go func() {
			o.Emit("done " + strconv.FormatUint(num, 10))
		}()
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}

	// blocks until listener is triggered numRoutines times
	<-doneChan
}

func TestBufferedEvents(t *testing.T) {
	var output []string
	var o = NewBufferedSetObserver(1 * time.Second)
	defer o.Close()

	done := make(chan bool)
	defer close(done)

	o.AddListener(func(ctx context.Context, e []string) {
		output = e
		done <- true
	})

	o.Emit("done1")
	o.Emit("done2")

	<-done // blocks until listener is triggered

	if len(output) != 2 {
		t.Error("error sending 2 buffered events.")
	}

	o.Emit("done")
	o.Emit("done")
	o.Emit("done")
	o.Emit("done")

	<-done // blocks until listener is triggered

	if len(output) != 1 {
		t.Error("error sending 4 buffered identical events.")
	}
}

type testObserver struct {
	mutex         sync.Mutex
	count         uint64
	countExpected uint64
	doneChan      chan<- interface{}
	chanClosed    bool
	err           error
}

func (o *testObserver) Observe(_ context.Context, e []string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	for range e {
		o.count++
		if o.chanClosed {
			o.err = errors.Errorf("received message after count reached expected(%d) received(%d)", o.countExpected, o.count)
			continue
		}
		if o.count == o.countExpected {
			o.doneChan <- struct{}{}
			close(o.doneChan)
			o.chanClosed = true
		}
	}
}
