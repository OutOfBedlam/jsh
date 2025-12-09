package global

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/dop251/goja"
)

type Subscribers map[EventType][]EventListener

type EventListener = goja.Callable

type objectID = string

func ObjectID(obj *goja.Object) objectID {
	return fmt.Sprintf("%p", obj)
}

type EventType = string

func GetEventLoop(vm *goja.Runtime) *EventLoop {
	if o := vm.Get("eventLoop"); o != nil {
		if el, ok := o.Export().(*EventLoop); ok {
			return el
		}
	}
	return nil
}

type EventLoop struct {
	vm          *goja.Runtime
	subscribers map[objectID]Subscribers
	subsMutex   sync.RWMutex // Protects subscribers map
	events      chan func()

	stateMutex          sync.RWMutex
	globalEventStops    []func()
	globalEventStopCh   chan struct{}
	globalEventStopOnce *sync.Once
	globalEventCloseCh  chan struct{}
	globalEventWg       sync.WaitGroup
}

func NewEventLoop(vm *goja.Runtime) *EventLoop {
	return &EventLoop{
		vm:          vm,
		subscribers: make(map[objectID]Subscribers),
		events:      make(chan func(), 100),
	}
}

// setSubscriber sets the event listener for the given object and event type,
// replacing any existing listeners.
func (el *EventLoop) setSubscriber(objectID objectID, eventType EventType, listener goja.Callable) {
	el.subsMutex.Lock()
	defer el.subsMutex.Unlock()

	if _, exists := el.subscribers[objectID]; !exists {
		el.subscribers[objectID] = make(Subscribers)
	}
	if listener == nil {
		delete(el.subscribers[objectID], eventType)
		return
	}
	el.subscribers[objectID][eventType] = []EventListener{listener}
}

// addSubscriber adds an event listener for the given object and event type,
// without replacing existing listeners.
func (el *EventLoop) addSubscriber(objectID objectID, eventType EventType, listener goja.Callable) {
	el.subsMutex.Lock()
	defer el.subsMutex.Unlock()

	if _, exists := el.subscribers[objectID]; !exists {
		el.subscribers[objectID] = make(Subscribers)
	}
	if listener == nil {
		delete(el.subscribers[objectID], eventType)
		return
	}
	el.subscribers[objectID][eventType] = append(el.subscribers[objectID][eventType], listener)
}

// getSubscribers retrieves the event listeners for the given object and event type.
func (el *EventLoop) getSubscribers(objectID objectID, eventType EventType) []EventListener {
	el.subsMutex.RLock()
	defer el.subsMutex.RUnlock()

	if objSubscribers, exists := el.subscribers[objectID]; exists {
		if listeners, exists := objSubscribers[eventType]; exists {
			// Return a copy to avoid race conditions
			copy := make([]EventListener, len(listeners))
			copy = append(copy[:0], listeners...)
			return copy
		}
	}
	return nil
}

// unsubscribeAll removes all event listeners for the given object.
func (el *EventLoop) unsubscribeAll(objectID objectID) {
	el.subsMutex.Lock()
	defer el.subsMutex.Unlock()
	delete(el.subscribers, objectID)
}

// publish triggers all event listeners for the given object and event type,
// passing the provided arguments to each listener.
func (el *EventLoop) publish(objectID objectID, eventType EventType, args ...goja.Value) {
	listeners := el.getSubscribers(objectID, eventType)
	if listeners == nil {
		return
	}

	xargs := make([]goja.Value, len(args))
	copy(xargs, args)
	for _, listener := range listeners {
		el.events <- func() {
			listener(goja.Undefined(), xargs...)
		}
	}
}

func (el *EventLoop) AddEventListener(obj *goja.Object, eventType EventType, listener goja.Callable) {
	el.addSubscriber(ObjectID(obj), eventType, listener)
}

func (el *EventLoop) DispatchEvent(obj *goja.Object, eventType EventType, args ...goja.Value) {
	el.publish(ObjectID(obj), eventType, args...)
}

func (el *EventLoop) Unregister(obj *goja.Object) {
	objID := ObjectID(obj)
	el.unsubscribeAll(objID)
}

// EventLoop registers start and stop functions for the global event loop.
// The obj parameter is the JavaScript object that will have event listener
// methods added to it. `addEventListener()` and `on()`.
func (el *EventLoop) Register(obj *goja.Object, start func(), stop func(), events []string) {
	if start != nil {
		el.events <- func() {
			el.globalEventWg.Add(1)
			go func(loopFunc func()) {
				defer el.globalEventWg.Done()
				loopFunc()
			}(start)
		}
	}
	if stop != nil {
		el.globalEventStops = append(el.globalEventStops, stop)
	}

	objID := ObjectID(obj)
	// usage: obj.addEventListener('eventType', handler)
	obj.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		if slices.Contains(events, eventType) == false {
			return el.vm.NewGoError(errors.New("unknown event type: " + eventType))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			return el.vm.NewGoError(errors.New("event handler must be a function"))
		}
		el.addSubscriber(objID, eventType, handler)
		return goja.Undefined()
	})
	// usage: obj.on('eventType', handler)
	obj.Set("on", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		if slices.Contains(events, eventType) == false {
			return el.vm.NewGoError(errors.New("unknown event type: " + eventType))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			return el.vm.NewGoError(errors.New("event handler must be a function"))
		}
		el.setSubscriber(objID, eventType, handler)
		return goja.Undefined()
	})
	// usage: obj.dispatchEvent('eventType', arg1, arg2, ...)
	obj.Set("dispatchEvent", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		if slices.Contains(events, eventType) == false {
			return el.vm.NewGoError(errors.New("unknown event type: " + eventType))
		}
		args := call.Arguments[1:]
		el.publish(objID, eventType, args...)
		return goja.Undefined()
	})
}

func (el *EventLoop) RunString(code string) (val goja.Value, err error) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	el.events <- func() {
		val, err = el.vm.RunString(code)
		wg.Done()
	}
	wg.Wait()
	return
}

func (el *EventLoop) RunProgram(prog *goja.Program) (val goja.Value, err error) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	el.events <- func() {
		val, err = el.vm.RunProgram(prog)
		wg.Done()
	}
	wg.Wait()
	return
}

func (el *EventLoop) Start() {
	el.stateMutex.Lock()
	if el.globalEventStopOnce != nil {
		el.stateMutex.Unlock()
		return // already started
	}
	el.globalEventStopOnce = &sync.Once{}
	el.globalEventStopCh = make(chan struct{})
	el.globalEventCloseCh = make(chan struct{})
	el.stateMutex.Unlock()
	defer func() {
		close(el.globalEventCloseCh)
		el.globalEventStopOnce.Do(func() {
			close(el.globalEventStopCh)
		})
	}()

	// doneCh := make(chan struct{})
	// go func() {
	// 	el.globalEventWg.Wait()
	// 	// close(doneCh)
	// }()

loop:
	for {
		select {
		// case <-doneCh:
		// 	// all event loops exited
		// 	break loop
		case <-el.globalEventStopCh:
			// stop signal received
			break loop
		case fn := <-el.events:
			fn()
		}
	}
	// drain remaining events
	for len(el.events) > 0 {
		fn := <-el.events
		fn()
	}

	slices.Reverse(el.globalEventStops)
	for _, stop := range el.globalEventStops {
		stop()
	}
}

func (el *EventLoop) Stop() {
	el.stateMutex.RLock()
	if el.globalEventStopOnce == nil {
		el.stateMutex.RUnlock()
		return // not started
	}
	stopOnce := el.globalEventStopOnce
	stopCh := el.globalEventStopCh
	closeCh := el.globalEventCloseCh
	el.stateMutex.RUnlock()

	stopOnce.Do(func() {
		if stopCh != nil {
			close(stopCh)
		}
	})

	if closeCh != nil {
		<-closeCh
	}
}

func (el *EventLoop) SetTimeout(fn goja.Callable, delay int, args ...goja.Value) goja.Value {
	ctx, cancel := context.WithCancel(context.Background())

	tm := el.vm.NewObject()
	el.Register(tm, func() {
		timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			el.DispatchEvent(tm, "expire", args...)
		case <-ctx.Done():
			// Timer was cancelled
		}
	}, func() {
		cancel()
	}, []string{"expire"})

	el.AddEventListener(tm, "expire", fn)
	el.AddEventListener(tm, "cancel", func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		cancel()
		return goja.Undefined(), nil
	})

	return tm
}

func (el *EventLoop) ClearTimeout(tm goja.Value) {
	el.DispatchEvent(tm.ToObject(el.vm), "cancel")
}
