# go-observer

Implementation of observer pattern with deduplication of events in the context of a time box for debouncing. 
Notification is handled in background via go routines.

## Usage

Example
```go
// new observer with 30 second debounce.  Duration is any valid time.Duration or if 0 then debounce is disabled.
observer := observer.NewBufferedSetObserver(30 * time.Second)

// observers should be closed before exiting the app
defer func() {
    if observer != nil {
        _ = observer.Close(ctx)
    }
}()

// add a listener function to be called when there are events, you can add any number of listeners
observer.AddListener(
    func() {(ctx context.Context, events []string) {
        // do something when you observe events
    }()
)

// emmit an event to all listeners
observer.Emit("some_string")
```

## Contributing

Happy to accept PRs.

# Author

**davidwartell**

* <http://github.com/davidwartell>
* <http://linkedin.com/in/wartell>
