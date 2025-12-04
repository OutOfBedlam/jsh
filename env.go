package jsh

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
)

// ExecBuilderFunc is a function that builds an *exec.Cmd given the source and arguments.
// if code is empty, it indicates that the file is being executed from file named in args[0].
// if code is non-empty, it indicates that the code is being executed.
type ExecBuilderFunc func(code string, args []string) (*exec.Cmd, error)

type Env interface {
	Reader() io.Reader
	Writer() io.Writer
	Filesystem() fs.FS
	ExecBuilder() ExecBuilderFunc
}

var _ Env = (*DefaultEnv)(nil)

func NewEnv(opts ...EnvOption) Env {
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

func WithExecBuilder(eb ExecBuilderFunc) EnvOption {
	return func(de *DefaultEnv) {
		de.execBuilder = eb
	}
}

type DefaultEnv struct {
	writer      io.Writer
	reader      io.Reader
	fs          fs.FS
	execBuilder ExecBuilderFunc
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

func (de *DefaultEnv) ExecBuilder() ExecBuilderFunc {
	return de.execBuilder
}
