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

		if el := global.GetEventLoop(rt); el != nil {
			el.Register(ws.obj, ws.Open, ws.Close, []string{
				"open", "close", "message", "error",
			})
		}

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

func (ws *WebSocket) fireEvent(eventType string, args ...goja.Value) {
	if el := global.GetEventLoop(ws.rt); el != nil {
		el.DispatchEvent(ws.obj, eventType, args...)
		return
	}
}

func (ws *WebSocket) Open() {
	if s, _, err := websocket.DefaultDialer.Dial(ws.addr, nil); err != nil {
		log.Printf("WebSocket connection error: %v", err)
		ws.fireEvent("error", ws.rt.NewGoError(err))
		return
	} else {
		ws.mu.Lock()
		ws.conn = s
		ws.mu.Unlock()
		ws.fireEvent("open", goja.Undefined())
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
				ws.fireEvent("close", ws.rt.ToValue(err.Error()))
			}
			return
		}
		data := map[string]any{}
		if typ == websocket.TextMessage {
			data["data"] = string(message)
		} else {
			data["data"] = message
		}
		data["type"] = typ
		ws.fireEvent("message", ws.rt.ToValue(data))
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
