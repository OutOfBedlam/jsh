package ws

import (
	_ "embed"
	"fmt"

	"github.com/OutOfBedlam/jsh/engine"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

//go:embed ws.js
var wsJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	m := rt.NewObject()
	m.Set("NewWebSocket", NewNativeWebSocket)
	rt.Set("_ws", m)

	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString(fmt.Sprintf(`(()=>{%s})()`, wsJS))
	if err != nil {
		panic(err)
	}
	rt.Set("module", goja.Undefined())
}

func NewNativeWebSocket(obj *goja.Object, url string, dispatch engine.EventDispatchFunc) *WebSocket {
	return &WebSocket{
		obj: obj,
		url: url,
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
	}
}

type WebSocket struct {
	obj  *goja.Object
	url  string
	emit func(event string, data any)
	conn *websocket.Conn
}

func (ws *WebSocket) Connect() error {
	if conn, _, err := websocket.DefaultDialer.Dial(ws.url, nil); err != nil {
		return err
	} else {
		ws.conn = conn
		go ws.run()
		return nil
	}
}

func (ws *WebSocket) Close() error {
	if ws.conn == nil {
		return nil
	}
	return ws.conn.Close()
}

func (ws *WebSocket) Send(typ int, message any) error {
	if ws.conn == nil {
		return fmt.Errorf("websocket is not open")
	}
	switch val := message.(type) {
	case string:
		return ws.conn.WriteMessage(typ, []byte(val))
	case []byte:
		return ws.conn.WriteMessage(typ, val)
	default:
		return fmt.Errorf("unsupported message type: %T", val)
	}
}

func (ws *WebSocket) run() {
	for {
		typ, message, err := ws.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				ws.emit("close", err)
			} else {
				ws.emit("error", err)
			}
			return
		}
		data := map[string]any{
			"type": typ,
		}
		if typ == websocket.TextMessage {
			data["data"] = string(message)
		} else {
			data["data"] = message
		}
		ws.emit("message", data)
	}
}
