package ws

import (
	"errors"
	"fmt"
	"io"
	"log"
	"slices"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	o.Set("WebSocket", newWebSocket(rt))
}

func newWebSocket(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("WebSocket constructor requires at least 1 argument"))
		}
		var addr = call.Argument(0).String()
		var options *goja.Object
		if len(call.Arguments) >= 2 {
			options = call.Argument(1).ToObject(rt)
		}
		ws := &WebSocket{
			rt:      rt,
			obj:     rt.NewObject(),
			addr:    addr,
			options: options,
		}
		ws.obj.Set("addEventListener", ws.addEventListener)
		ws.obj.Set("on", ws.on)
		ws.obj.Set("close", ws.Close)
		ws.obj.Set("send", ws.send)

		global.EventLoopAdd(ws.Open, ws.Close)

		return ws.obj
	}
}

type WebSocket struct {
	rt      *goja.Runtime
	obj     *goja.Object
	addr    string
	options *goja.Object
	conn    *websocket.Conn
}

var EventTypes []string = []string{
	"open",
	"message",
	"error",
	"close",
}

type Event struct {
	Type string
	Data goja.Value
}

func ErrorOrUndefined(err error) goja.Value {
	if err != nil {
		return goja.New().NewGoError(err)
	}
	return goja.Undefined()
}

func (ws *WebSocket) addEventListener(call goja.FunctionCall) goja.Value {
	eventType := call.Argument(0).String()
	if slices.Contains(EventTypes, eventType) == false {
		return ws.rt.NewGoError(errors.New("unknown event type: " + eventType))
	}
	handler, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		return ws.rt.NewGoError(errors.New("event handler must be a function"))
	}
	global.AddSubscriber(global.ObjectID(ws.obj), eventType, handler)
	return goja.Undefined()
}

func (ws *WebSocket) on(call goja.FunctionCall) goja.Value {
	eventType := call.Argument(0).String()
	if slices.Contains(EventTypes, eventType) == false {
		return ws.rt.NewGoError(errors.New("unknown event type: " + eventType))
	}
	handler, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		return ws.rt.NewGoError(errors.New("event handler must be a function"))
	}
	global.SetSubscriber(global.ObjectID(ws.obj), eventType, handler)
	return goja.Undefined()
}

func (ws *WebSocket) Send(data string) error {
	return ws.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

func (ws *WebSocket) send(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		return goja.Undefined()
	}
	var data string
	switch v := call.Argument(0).Export().(type) {
	case string:
		data = v
	case []byte:
		data = string(v)
	default:
		data = fmt.Sprintf("%v", v)
	}
	return ErrorOrUndefined(ws.Send(data))
}

func (ws *WebSocket) Open() {
	if s, _, err := websocket.DefaultDialer.Dial(ws.addr, nil); err != nil {
		log.Printf("WebSocket connection error: %v", err)
		global.Publish(global.ObjectID(ws.obj), "error", ws.rt.NewGoError(err))
		return
	} else {
		ws.conn = s
		global.Publish(global.ObjectID(ws.obj), "open", goja.Undefined())
	}

	for {
		typ, message, err := ws.conn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				global.Publish(global.ObjectID(ws.obj), "close", ws.rt.ToValue(err.Error()))
			}
			return
		}
		data := ws.rt.NewObject()
		data.Set("data", ws.rt.ToValue(message))
		data.Set("type", ws.rt.ToValue(typ))
		global.Publish(global.ObjectID(ws.obj), "message", data)
	}
}

func (ws *WebSocket) Close() {
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
}
