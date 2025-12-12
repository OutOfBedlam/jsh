package ws

import (
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/gorilla/websocket"
)

//go:embed ws.js
var wsJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	module.Set("NewWebSocket", NewNativeWebSocket(rt))

	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString(fmt.Sprintf(`(()=>{%s})()`, wsJS))
	if err != nil {
		panic(err)
	}
}

func NewNativeWebSocket(vm *goja.Runtime) func(obj *goja.Object) *WebSocket {
	loop := vm.Get("eventloop").Export().(*eventloop.EventLoop)
	return func(obj *goja.Object) *WebSocket {
		return &WebSocket{
			obj:  obj,
			loop: loop,
			addr: obj.Get("url").String(),
		}
	}
}

type WebSocket struct {
	obj  *goja.Object
	loop *eventloop.EventLoop

	addr   string
	mu     sync.RWMutex
	conn   *websocket.Conn
	closed bool
	vital  *eventloop.Interval
}

func (ws *WebSocket) emit(eventType string, args ...any) {
	global.Emit(ws.loop, ws.obj, eventType, args...)
}

func (ws *WebSocket) Open() {
	ws.vital = ws.loop.SetInterval(func(r *goja.Runtime) {
		if ws.closed {
			ws.loop.ClearInterval(ws.vital)
		}
	}, 500*time.Millisecond)

	go ws.run()
}

func (ws *WebSocket) run() {
	defer ws.loop.ClearInterval(ws.vital)
	if s, _, err := websocket.DefaultDialer.Dial(ws.addr, nil); err != nil {
		ws.closed = true
		ws.emit("error", err)
		return
	} else {
		ws.mu.Lock()
		ws.conn = s
		ws.mu.Unlock()
		ws.emit("open")
	}

	for {
		ws.mu.RLock()
		conn := ws.conn
		ws.mu.RUnlock()

		if conn == nil {
			ws.emit("connection error: nil conn")
			return
		}

		typ, message, err := conn.ReadMessage()
		if err != nil {
			if ws.closed || !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				ws.emit("close", err)
			} else {
				ws.emit("error", err)
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
		ws.emit("message", data)
	}
}

func (ws *WebSocket) Close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.closed = true
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
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
