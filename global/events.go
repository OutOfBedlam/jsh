package global

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

func Emit(loop *eventloop.EventLoop, obj *goja.Object, eventType string, args ...any) {
	loop.RunOnLoop(func(vm *goja.Runtime) {
		values := make([]goja.Value, len(args))
		for i, a := range args {
			values[i] = vm.ToValue(a)
		}

		obj.Get("emit").Export().(func(goja.FunctionCall) goja.Value)(goja.FunctionCall{
			This:      obj,
			Arguments: append([]goja.Value{vm.ToValue(eventType)}, values...),
		})
	})
}
