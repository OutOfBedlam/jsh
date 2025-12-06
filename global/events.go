package global

import (
	"errors"
	"slices"
	"sync"

	"github.com/dop251/goja"
)

var subscribers map[objectID]Subscribers = make(map[objectID]Subscribers)

type Subscribers map[EventType][]EventListener

type EventListener = goja.Callable

type objectID = string

func ObjectID(obj *goja.Object) objectID {
	return obj.String()
}

type EventType = string

// SetSubscriber sets the event listener for the given object and event type,
// replacing any existing listeners.
func SetSubscriber(objectID objectID, eventType EventType, listener goja.Callable) {
	if _, exists := subscribers[objectID]; !exists {
		subscribers[objectID] = make(Subscribers)
	}
	if listener == nil {
		delete(subscribers[objectID], eventType)
	}
	subscribers[objectID][eventType] = []EventListener{listener}
}

// AddSubscriber adds an event listener for the given object and event type,
// without replacing existing listeners.
func AddSubscriber(objectID objectID, eventType EventType, listener goja.Callable) {
	if _, exists := subscribers[objectID]; !exists {
		subscribers[objectID] = make(Subscribers)
	}
	if listener == nil {
		delete(subscribers[objectID], eventType)
	}
	subscribers[objectID][eventType] = append(subscribers[objectID][eventType], listener)
}

// GetSubscribers retrieves the event listeners for the given object and event type.
func GetSubscribers(objectID objectID, eventType EventType) []EventListener {
	if objSubscribers, exists := subscribers[objectID]; exists {
		if listeners, exists := objSubscribers[eventType]; exists {
			return listeners
		}
	}
	return nil
}

// UnsubscribeAll removes all event listeners for the given object.
func UnsubscribeAll(objectID objectID) {
	delete(subscribers, objectID)
}

// Publish triggers all event listeners for the given object and event type,
// passing the provided arguments to each listener.
func Publish(objectID objectID, eventType EventType, args ...goja.Value) {
	if listeners := GetSubscribers(objectID, eventType); listeners != nil {
		for _, listener := range listeners {
			listener(goja.Undefined(), args...)
		}
	}
}

// EventLoop registers start and stop functions for the global event loop.
// The obj parameter is the JavaScript object that will have event listener
// methods added to it. `addEventListener()` and `on()`.
func EventLoop(obj *goja.Object, vm *goja.Runtime, events []string, start func(), stop func()) {
	if start != nil {
		globalEventStarts = append(globalEventStarts, start)
	}
	if stop != nil {
		globalEventStops = append(globalEventStops, stop)
	}

	// obj.addEventListener('eventType', handler)
	obj.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		if slices.Contains(events, eventType) == false {
			return vm.NewGoError(errors.New("unknown event type: " + eventType))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			return vm.NewGoError(errors.New("event handler must be a function"))
		}
		AddSubscriber(ObjectID(obj), eventType, handler)
		return goja.Undefined()
	})
	// obj.on('eventType', handler)
	obj.Set("on", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		if slices.Contains(events, eventType) == false {
			return vm.NewGoError(errors.New("unknown event type: " + eventType))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			return vm.NewGoError(errors.New("event handler must be a function"))
		}
		SetSubscriber(ObjectID(obj), eventType, handler)
		return goja.Undefined()
	})
}

var globalEventStarts []func()
var globalEventStops []func()
var globalEventStopCh chan struct{}
var globalEventStopOnce *sync.Once
var globalEventCloseCh chan struct{}

func EventLoopStart() {
	if globalEventStopOnce != nil {
		return // already started
	}
	globalEventStopOnce = &sync.Once{}
	globalEventStopCh = make(chan struct{})
	globalEventCloseCh = make(chan struct{})
	defer func() {
		close(globalEventCloseCh)
		globalEventStopOnce.Do(func() {
			close(globalEventStopCh)
		})
	}()

	wg := sync.WaitGroup{}
	for _, loop := range globalEventStarts {
		wg.Add(1)
		go func(loopFunc func()) {
			defer wg.Done()
			loopFunc()
		}(loop)
	}
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// all event loops exited
	case <-globalEventStopCh:
		// stop signal received
	}

	slices.Reverse(globalEventStops)
	for _, stop := range globalEventStops {
		stop()
	}
}

func EventLoopStop() {
	if globalEventStopOnce == nil {
		return // not started
	}
	globalEventStopOnce.Do(func() {
		close(globalEventStopCh)
	})
	<-globalEventCloseCh
}

// EventLoopReset clears all registered event loop functions.
// This is purposed for testing to ensure a clean state between tests.
func EventLoopReset() {
	globalEventStopOnce = nil
	globalEventStarts = nil
	globalEventStops = nil
	subscribers = make(map[objectID]Subscribers)
}
