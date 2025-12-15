package ws

import (
	_ "embed"
	"fmt"

	"github.com/OutOfBedlam/jsh"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/gorilla/websocket"
)

//go:embed ws.js
var wsJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	module.Set("NewWebSocket", NewNativeWebSocket)

	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString(fmt.Sprintf(`(()=>{%s})()`, wsJS))
	if err != nil {
		panic(err)
	}
}

func NewNativeWebSocket(obj *goja.Object) *WebSocket {
	return &WebSocket{
		obj: obj,
	}
}

type WebSocket struct {
	obj *goja.Object
}

func (ws *WebSocket) Connect(addr string) (*websocket.Conn, error) {
	if s, _, err := websocket.DefaultDialer.Dial(addr, nil); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (ws *WebSocket) Send(conn *websocket.Conn, typ int, message any) error {
	switch val := message.(type) {
	case string:
		return conn.WriteMessage(typ, []byte(val))
	case []byte:
		return conn.WriteMessage(typ, val)
	default:
		return fmt.Errorf("unsupported message type: %T", val)
	}
}

func (ws *WebSocket) emit(loop *eventloop.EventLoop, eventType string, args ...any) {
	jsh.Emit(loop, ws.obj, eventType, args...)
}

func (ws *WebSocket) Start(conn *websocket.Conn, loop *eventloop.EventLoop) {
	go ws.run(conn, loop)
}

func (ws *WebSocket) run(conn *websocket.Conn, loop *eventloop.EventLoop) {
	for {
		typ, message, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				ws.emit(loop, "close", err)
			} else {
				ws.emit(loop, "error", err)
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
		ws.emit(loop, "message", data)
	}
}
