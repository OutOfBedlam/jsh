package root

import (
	"embed"
	"io/fs"

	"github.com/OutOfBedlam/jsh/engine"
)

//go:embed embed/*
var rootFS embed.FS

func RootFSTab() engine.FSTab {
	dirfs, _ := fs.Sub(rootFS, "embed")
	return engine.FSTab{MountPoint: "/", FS: dirfs}
}
