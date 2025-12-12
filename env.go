package jsh

import (
	"bytes"
	"io"
	"io/fs"
	"os"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/dop251/goja_nodejs/require"
)

type DefaultEnv struct {
	writer      io.Writer
	reader      io.Reader
	fs          fs.FS
	execBuilder global.ExecBuilderFunc
	vars        map[string]any
	natives     map[string]require.ModuleLoader
}

var _ global.Env = (*DefaultEnv)(nil)
var _ global.EnvFS = (*DefaultEnv)(nil)
var _ global.EnvExec = (*DefaultEnv)(nil)
var _ global.EnvNativeModule = (*DefaultEnv)(nil)

func NewEnv(opts ...EnvOption) global.Env {
	ret := &DefaultEnv{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type EnvOption func(*DefaultEnv)

func WithFilesystem(fs fs.FS) EnvOption {
	return func(de *DefaultEnv) {
		de.fs = fs
	}
}

func WithWriter(w io.Writer) EnvOption {
	return func(de *DefaultEnv) {
		de.writer = w
	}
}

func WithReader(r io.Reader) EnvOption {
	return func(de *DefaultEnv) {
		de.reader = r
	}
}

func WithExecBuilder(eb global.ExecBuilderFunc) EnvOption {
	return func(de *DefaultEnv) {
		de.execBuilder = eb
	}
}

func WithNativeModule(name string, loader require.ModuleLoader) EnvOption {
	return func(de *DefaultEnv) {
		if de.natives == nil {
			de.natives = make(map[string]require.ModuleLoader)
		}
		de.natives[name] = loader
	}
}

func (de *DefaultEnv) Reader() io.Reader {
	if de.reader != nil {
		return de.reader
	}
	return os.Stdin
}

func (de *DefaultEnv) Writer() io.Writer {
	if de.writer != nil {
		return de.writer
	}
	return os.Stdout
}

func (de *DefaultEnv) Filesystem() fs.FS {
	return de.fs
}

func (de *DefaultEnv) ExecBuilder() global.ExecBuilderFunc {
	return de.execBuilder
}

func (de *DefaultEnv) Set(key string, value any) {
	if de.vars == nil {
		de.vars = make(map[string]any)
	}
	if value == nil {
		delete(de.vars, key)
		return
	}
	de.vars[key] = value
}

func (de *DefaultEnv) Get(key string) any {
	if de.vars == nil {
		return nil
	}
	return de.vars[key]
}

func (de *DefaultEnv) NativeModules() map[string]require.ModuleLoader {
	return de.natives
}

type TestEnv struct {
	Input           bytes.Buffer
	Output          bytes.Buffer
	ExecBuilderFunc global.ExecBuilderFunc
	Mounts          map[string]fs.FS
	Natives         map[string]require.ModuleLoader
	Vars            map[string]any
}

var _ global.Env = (*TestEnv)(nil)
var _ global.EnvNativeModule = (*TestEnv)(nil)

func (te *TestEnv) Reader() io.Reader {
	return &te.Input
}

func (te *TestEnv) Writer() io.Writer {
	return &te.Output
}

func (te *TestEnv) Filesystem() fs.FS {
	fileSystem := NewFS()
	for dir, subFileSystem := range te.Mounts {
		fileSystem.Mount(dir, subFileSystem)
	}
	return fileSystem
}

func (te *TestEnv) ExecBuilder() global.ExecBuilderFunc {
	return te.ExecBuilderFunc
}

func (te *TestEnv) Set(key string, value any) {
	if te.Vars == nil {
		te.Vars = make(map[string]any)
	}
	te.Vars[key] = value
}

func (te *TestEnv) Get(key string) any {
	switch key {
	case "PATH":
		return "/:/sbin:/work"
	default:
		return te.Vars[key]
	}
}

func (te *TestEnv) NativeModules() map[string]require.ModuleLoader {
	return te.Natives
}
