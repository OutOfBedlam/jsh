package jsh

import (
	_ "embed"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

//go:embed events.js
var eventsJS string

func NewEventLoop(opts ...eventloop.Option) *eventloop.EventLoop {
	loop := eventloop.NewEventLoop(opts...)
	loop.Run(func(vm *goja.Runtime) {
		if _, err := vm.RunString(eventsJS); err != nil {
			panic(err)
		}
	})
	return loop
}

type EventDispatchFunc func(obj *goja.Object, event string, args ...any)

func dispatchEvent(loop *eventloop.EventLoop) EventDispatchFunc {
	return func(obj *goja.Object, event string, args ...any) {
		loop.RunOnLoop(func(vm *goja.Runtime) {
			values := make([]goja.Value, len(args))
			for i, a := range args {
				values[i] = vm.ToValue(a)
			}
			if emit, ok := obj.Get("emit").Export().(func(goja.FunctionCall) goja.Value); ok {
				emit(goja.FunctionCall{
					This:      obj,
					Arguments: append([]goja.Value{vm.ToValue(event)}, values...),
				})
			}
		})
	}
}
