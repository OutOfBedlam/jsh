package ws

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

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
		ws.obj.Set("close", ws.Close)
		ws.obj.Set("send", ws.send)

		events := []string{
			"open", "close", "message", "error",
		}
		global.EventLoop(ws.obj, rt, events, ws.Open, ws.Close)

		return ws.obj
	}
}

type WebSocket struct {
	rt      *goja.Runtime
	obj     *goja.Object
	addr    string
	options *goja.Object
	conn    *websocket.Conn
	mu      sync.RWMutex
}

func ErrorOrUndefined(err error) goja.Value {
	if err != nil {
		return goja.New().NewGoError(err)
	}
	return goja.Undefined()
}

func (ws *WebSocket) Send(data string) error {
	ws.mu.RLock()
	conn := ws.conn
	ws.mu.RUnlock()
	if conn == nil {
		return errors.New("websocket connection is closed")
	}
	return conn.WriteMessage(websocket.TextMessage, []byte(data))
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
		ws.mu.Lock()
		ws.conn = s
		ws.mu.Unlock()
		global.Publish(global.ObjectID(ws.obj), "open", goja.Undefined())
	}

	for {
		ws.mu.RLock()
		conn := ws.conn
		ws.mu.RUnlock()

		if conn == nil {
			return
		}

		typ, message, err := conn.ReadMessage()
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
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
}
