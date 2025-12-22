package native

import (
	"embed"
	"io/fs"

	"github.com/OutOfBedlam/jsh/engine"
	"github.com/OutOfBedlam/jsh/native/http"
	"github.com/OutOfBedlam/jsh/native/mqtt"
	"github.com/OutOfBedlam/jsh/native/readline"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/OutOfBedlam/jsh/native/ws"
)

//go:embed root/*
var rootFS embed.FS

func RootFSTab() engine.FSTab {
	dirfs, _ := fs.Sub(rootFS, "root")
	return engine.FSTab{MountPoint: "/", FS: dirfs}
}

func ConfigureRoot(c *engine.Config) {
	c.AddFSTabHook(func(tabs engine.FSTabs) engine.FSTabs {
		if !tabs.HasMountPoint("/") {
			tabs = append([]engine.FSTab{RootFSTab()}, tabs...)
		}
		return tabs
	})
}

func Enable(n *engine.JSRuntime) {
	n.RegisterNativeModule("@jsh/process", n.Process)
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	n.RegisterNativeModule("@jsh/readline", readline.Module)
	n.RegisterNativeModule("@jsh/http", http.Module)
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	n.RegisterNativeModule("@jsh/mqtt", mqtt.Module)
}
