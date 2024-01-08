package observer

import (
	"context"
	"sync"
	"time"
)

// Listener is the function type to run on events.
type Listener func(context.Context, []string)

// BufferedSetObserver implements the observer pattern with debounce.
// BufferedSetObserver delivers messages to observers. If bufferDuration > 0 then there is a delay in message deliver in
// which time new messages are added to a set eliminating duplicates.
type BufferedSetObserver struct {
	sync.RWMutex   // protects events, bufferDuration
	bufferDuration time.Duration
	events         chan<- string

	ctx     context.Context
	cancel  context.CancelFunc
	ctxDone <-chan struct{}
	wg      sync.WaitGroup

	listenersLock sync.Mutex // protects listeners
	listeners     []Listener

	bufferEventsLock sync.Mutex // protects bufferEvents, bufferReceived
	bufferEvents     map[string]struct{}
	bufferReceived   uint64 // number of events including duplicates sent to the current buffer since last flush
}

// NewBufferedSetObserver
// Creates new instance of BufferedSetObserver.
// If bufferDuration is 0 then the message is delivered immediately to all observers.
func NewBufferedSetObserver(bufferDuration time.Duration) *BufferedSetObserver {
	eventsChan := make(chan string)
	obs := &BufferedSetObserver{
		events:         eventsChan,
		bufferDuration: bufferDuration,
		bufferEvents:   make(map[string]struct{}),
		bufferReceived: 0,
	}
	obs.ctx, obs.cancel = context.WithCancel(context.Background())
	obs.ctxDone = obs.ctx.Done()
	obs.eventLoop(eventsChan)
	return obs
}

// Close the observer channels,
func (o *BufferedSetObserver) Close() {
	o.Lock()
	defer o.Unlock()

	if o.cancel != nil {
		o.cancel()
	}

	// Wait for all of the go routines delivering messages to exit.
	o.wg.Wait()

	// close the channel after all readers and writers have exited
	if o.events != nil {
		close(o.events)
		o.events = nil
	}
}

// AddListener adds a listener function to run on message delivery with the spec:
// listener(events []string)
func (o *BufferedSetObserver) AddListener(listener Listener) {
	o.listenersLock.Lock()
	defer o.listenersLock.Unlock()
	o.listeners = append(o.listeners, listener)
}

// Emit an event, when event is triggered all
// listeners will be called using the event string.
func (o *BufferedSetObserver) Emit(event string) {
	eventClone := make([]byte, len(event))
	copy(eventClone, event)
	o.RLock()
	o.wg.Add(1)
	defer o.wg.Done()
	if o.events == nil {
		o.RUnlock()
		return
	}
	eventChan := o.events
	o.RUnlock()

	eventChan <- string(eventClone)
}

// SetBufferDuration set the event buffer damping duration.
func (o *BufferedSetObserver) SetBufferDuration(d time.Duration) {
	o.Lock()
	defer o.Unlock()
	o.bufferDuration = d
}

// sendEvent send one or more events to the observer listeners.
func (o *BufferedSetObserver) sendEvent(events []string) {
	o.listenersLock.Lock()
	defer o.listenersLock.Unlock()
	for _, listener := range o.listeners {
		if o.ctx.Err() != nil {
			break
		}
		goListener := listener
		o.wg.Add(1)
		go func() {
			goListener(o.ctx, events)
			o.wg.Done()
		}()
	}
}

// handleEvent handle an event.
func (o *BufferedSetObserver) handleEvent(event string) {
	o.RLock()
	bufferDuration := o.bufferDuration
	o.RUnlock()

	// If we do not buffer events, just send this event now.
	if bufferDuration == 0 {
		o.sendEvent([]string{event})
		return
	}

	// Add new event to the event buffer.
	o.bufferEventsLock.Lock()
	o.bufferEvents[event] = struct{}{}
	o.bufferReceived++
	isFirstEvent := o.bufferReceived == 1
	o.bufferEventsLock.Unlock()

	// If this is the first event, set a timeout function.
	if isFirstEvent {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			for {
				select {
				case <-time.After(bufferDuration):
					o.bufferEventsLock.Lock()

					// Send all events in event buffer.
					var events []string
					for k := range o.bufferEvents {
						events = append(events, k)
					}
					o.sendEvent(events)

					// Reset events buffer.
					o.bufferEvents = make(map[string]struct{})
					o.bufferReceived = 0

					o.bufferEventsLock.Unlock()
					return

				case <-o.ctxDone:
					return
				}
			}
		}()
	}
}

// eventLoop runs the event loop.
func (o *BufferedSetObserver) eventLoop(eventsChan <-chan string) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
	readChannelsLoop:
		for {
			select {
			case event := <-eventsChan:
				o.handleEvent(event)

			case <-o.ctxDone:
				break readChannelsLoop
			}
		}
		// drain the channel to unblock writers
		for len(eventsChan) > 0 {
			<-eventsChan
		}
	}()
}
