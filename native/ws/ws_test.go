package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// echoServer is a simple WebSocket echo server for testing
func echoServer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if err := conn.WriteMessage(messageType, message); err != nil {
			break
		}
	}
}

// broadcastServer sends a message to clients immediately after connection
func broadcastServer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send initial message
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello from server")); err != nil {
		return
	}

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func TestWebSocketModule(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)

	Module(rt, module)

	if exports.Get("WebSocket") == nil {
		t.Fatal("WebSocket constructor not exported")
	}
}

func TestWebSocketConstructor(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)

	// Test with no arguments
	_, err := rt.RunString(`
		const WS = exports.WebSocket;
		try {
			new WS();
		} catch(e) {
			throw e;
		}
	`)
	if err == nil {
		t.Error("Expected error when constructing WebSocket without arguments")
	}

	// Test with valid URL
	_, err = rt.RunString(`
		const ws = new exports.WebSocket("ws://localhost:8080");
	`)
	if err != nil {
		t.Errorf("Failed to construct WebSocket with valid URL: %v", err)
	}
}

func TestWebSocketConnection(t *testing.T) {
	// Reset event loop state before test
	global.EventLoopReset()

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)
	rt.Set("testURL", wsURL)

	opened := make(chan bool, 1)
	rt.Set("onOpen", func() {
		opened <- true
	})

	_, err := rt.RunString(`
		const WS = exports.WebSocket;
		const ws = new WS(testURL);
		ws.on("open", onOpen);
	`)
	if err != nil {
		t.Fatalf("Failed to setup WebSocket: %v", err)
	}

	// Start event loop
	go global.EventLoopStart()
	defer global.EventLoopStop()

	select {
	case <-opened:
		// Connection opened successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for WebSocket connection to open")
	}
}

func TestWebSocketSendReceive(t *testing.T) {
	// Reset event loop state before test
	global.EventLoopReset()

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)
	rt.Set("testURL", wsURL)

	received := make(chan string, 1)
	rt.Set("onMessage", func(data goja.Value) {
		obj := data.ToObject(rt)
		if msgData := obj.Get("data"); msgData != nil {
			received <- string(msgData.Export().([]byte))
		}
	})

	_, err := rt.RunString(`
		const ws = new exports.WebSocket(testURL);
		ws.on("open", function() {
			ws.send("test message");
		});
		ws.on("message", onMessage);
	`)
	if err != nil {
		t.Fatalf("Failed to setup WebSocket: %v", err)
	}

	// Start event loop
	go global.EventLoopStart()
	defer global.EventLoopStop()

	select {
	case msg := <-received:
		if msg != "test message" {
			t.Errorf("Expected 'test message', got '%s'", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for echo message")
	}
}

func TestWebSocketClose(t *testing.T) {
	// Reset event loop state before test
	global.EventLoopReset()

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)
	rt.Set("testURL", wsURL)

	closed := make(chan bool, 1)
	rt.Set("onClose", func() {
		closed <- true
	})

	_, err := rt.RunString(`
		const WS = exports.WebSocket;
		const ws = new WS(testURL);
		ws.on("open", function() {
			ws.close();
		});
		ws.on("close", onClose);
	`)
	if err != nil {
		t.Fatalf("Failed to setup WebSocket: %v", err)
	}

	// Start event loop
	go global.EventLoopStart()
	defer global.EventLoopStop()

	// Note: close event might not fire immediately, so we give it some time
	time.Sleep(500 * time.Millisecond)
}

func TestWebSocketMultipleEventListeners(t *testing.T) {
	// Reset event loop state before test
	global.EventLoopReset()

	server := httptest.NewServer(http.HandlerFunc(broadcastServer))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)
	rt.Set("testURL", wsURL)

	counter := 0
	rt.Set("incrementCounter", func() {
		counter++
	})

	_, err := rt.RunString(`
		const WS = exports.WebSocket;
		const ws = new WS(testURL);
		ws.addEventListener("message", incrementCounter);
		ws.addEventListener("message", incrementCounter);
	`)
	if err != nil {
		t.Fatalf("Failed to setup WebSocket: %v", err)
	}

	// Start event loop
	go global.EventLoopStart()
	defer global.EventLoopStop()

	time.Sleep(500 * time.Millisecond)

	if counter != 2 {
		t.Errorf("Expected counter to be 2, got %d", counter)
	}
}

func TestWebSocketInvalidEventType(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)

	errorCaught := false
	rt.Set("checkError", func(val goja.Value) {
		if val != goja.Undefined() {
			errorCaught = true
		}
	})

	_, err := rt.RunString(`
		const ws = new exports.WebSocket("ws://localhost:8080");
		const result = ws.on("invalid_event", function() {});
		checkError(result);
	`)
	if err != nil {
		// JavaScript error thrown
		return
	}
	if !errorCaught {
		t.Error("Expected error when adding listener for invalid event type")
	}
}

func TestWebSocketConnectionError(t *testing.T) {
	// Reset event loop state before test
	global.EventLoopReset()

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(rt, module)

	rt.Set("exports", exports)

	errorReceived := make(chan bool, 1)
	rt.Set("onError", func() {
		errorReceived <- true
	})

	// Try to connect to a non-existent server
	_, err := rt.RunString(`
		const WS = exports.WebSocket;
		const ws = new WS("ws://localhost:9999");
		ws.on("error", onError);
	`)
	if err != nil {
		t.Fatalf("Failed to setup WebSocket: %v", err)
	}

	// Start event loop
	go global.EventLoopStart()
	defer global.EventLoopStop()

	select {
	case <-errorReceived:
		// Error received as expected
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection error")
	}
}

func TestWebSocketSendWithoutConnection(t *testing.T) {
	rt := goja.New()
	ws := &WebSocket{
		rt:   rt,
		addr: "ws://localhost:8080",
		conn: nil,
	}

	// Send should fail with nil connection
	err := ws.Send("test")
	if err == nil {
		t.Error("Expected error when sending without connection")
	}
}

func TestErrorOrUndefined(t *testing.T) {
	result := ErrorOrUndefined(nil)
	if !goja.IsUndefined(result) {
		t.Error("Expected undefined for nil error")
	}

	result = ErrorOrUndefined(http.ErrServerClosed)
	if goja.IsUndefined(result) {
		t.Error("Expected error value for non-nil error")
	}
}
