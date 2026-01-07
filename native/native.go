package native

import (
	"github.com/OutOfBedlam/jsh/engine"
	"github.com/OutOfBedlam/jsh/native/http"
	"github.com/OutOfBedlam/jsh/native/mqtt"
	"github.com/OutOfBedlam/jsh/native/net"
	"github.com/OutOfBedlam/jsh/native/readline"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/OutOfBedlam/jsh/native/stream"
	"github.com/OutOfBedlam/jsh/native/ws"
)

func Enable(n *engine.JSRuntime) {
	n.RegisterNativeModule("@jsh/process", n.Process)
	n.RegisterNativeModule("@jsh/fs", n.Filesystem)
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	n.RegisterNativeModule("@jsh/readline", readline.Module)
	n.RegisterNativeModule("@jsh/http", http.Module)
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	n.RegisterNativeModule("@jsh/mqtt", mqtt.Module)
	n.RegisterNativeModule("@jsh/stream", stream.Module)
	n.RegisterNativeModule("@jsh/net", net.Module)
}
