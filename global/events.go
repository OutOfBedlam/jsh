package global

import (
	"slices"

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

// EventLoopAdd registers start and stop functions for the global event loop.
func EventLoopAdd(start func(), stop func()) {
	globalEventStarts = append(globalEventStarts, start)
	globalEventStops = append(globalEventStops, stop)
}

var globalEventStarts []func()
var globalEventStops []func()
var globalEventCh chan struct{}
var globalEventCloseCh chan struct{}

func EventLoopStart() {
	globalEventCh = make(chan struct{})
	globalEventCloseCh = make(chan struct{})
	defer close(globalEventCloseCh)

	for _, loop := range globalEventStarts {
		go loop()
	}

	<-globalEventCh

	slices.Reverse(globalEventStops)
	for _, stop := range globalEventStops {
		stop()
	}
}

func EventLoopStop() {
	close(globalEventCh)
	<-globalEventCloseCh
}
