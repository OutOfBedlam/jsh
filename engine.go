package jsh

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/OutOfBedlam/jsh/log"
	"github.com/dop251/goja"
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

type JSRuntime struct {
	Name   string
	Source string
	Args   []string
	Strict bool
	Env    global.Env

	vm            *goja.Runtime
	exitCode      int
	shutdownHooks []func()
	nowFunc       func() time.Time
}

func (jr *JSRuntime) Run() error {
	if jr.Env == nil {
		jr.Env = &DefaultEnv{}
	}

	jr.vm = goja.New()
	jr.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	registry := require.NewRegistry(
		require.WithGlobalFolders("node_modules"),
		require.WithLoader(jr.loadSource),
	)
	if envNM, ok := jr.Env.(global.EnvNativeModule); ok {
		for name, loader := range envNM.NativeModules() {
			registry.RegisterNativeModule(name, loader)
		}
	}
	registry.Enable(jr.vm)

	jr.vm.Set("console", log.SetConsole(jr.vm, jr.Env.Writer()))
	jr.vm.Set("now", jr.doNow)

	eventLoop := global.NewEventLoop(jr.vm)
	jr.vm.Set("eventLoop", eventLoop)
	jr.vm.Set("setTimeout", eventLoop.SetTimeout)
	jr.vm.Set("clearTimeout", eventLoop.ClearTimeout)
	go eventLoop.Start()
	defer eventLoop.Stop()

	obj := jr.vm.NewObject()
	jr.vm.Set("runtime", obj)
	obj.Set("addShutdownHook", jr.doAddShutdownHook)
	obj.Set("exit", jr.doExit)
	obj.Set("env", jr.Env)
	obj.Set("args", jr.doArgs())
	obj.Set("exec", jr.doExec)
	obj.Set("execString", jr.doExecString)

	// guarantee shutdown hooks run at the end
	defer func() {
		slices.Reverse(jr.shutdownHooks)
		for _, hook := range jr.shutdownHooks {
			hook()
		}
	}()

	program, err := goja.Compile(jr.Name, jr.Source, jr.Strict)
	if err != nil {
		return err
	}

	if _, err := eventLoop.RunProgram(program); err != nil {
		jr.exitCode = -1
		return err
	}
	return nil
}

func (jr *JSRuntime) ExitCode() int {
	return jr.exitCode
}

func (jr *JSRuntime) loadSource(moduleName string) ([]byte, error) {
	return global.LoadSource(jr.Env, moduleName)
}

func (jr *JSRuntime) doAddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

type Exit struct {
	Code int
}

func (jr *JSRuntime) doExit(call goja.FunctionCall) goja.Value {
	exit := Exit{Code: 0}
	if len(call.Arguments) > 0 {
		exit.Code = int(call.Argument(0).ToInteger())
	}
	jr.vm.Interrupt(exit)
	return goja.Undefined()
}

func (jr *JSRuntime) doNow() goja.Value {
	now := jr.nowFunc
	if now == nil {
		now = time.Now
	}
	return jr.vm.ToValue(now())
}

func (jr *JSRuntime) doArgs() goja.Value {
	s := make([]any, len(jr.Args))
	for i, arg := range jr.Args {
		s[i] = arg
	}
	return jr.vm.NewArray(s...)
}

// doExecString executes a command line string via the exec function.
//
// syntax) execString(source: string, ...args: string): number
// return) exit code
func (jr *JSRuntime) doExecString(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		return jr.vm.NewGoError(fmt.Errorf("no source code provided"))
	}
	args := []string{call.Arguments[0].String()}
	for i := 1; i < len(call.Arguments); i++ {
		args = append(args, call.Arguments[i].String())
	}
	return jr.exec(args[0], args[1:])
}

// doExec executes a command by building an exec.Cmd and running it.
//
// syntax) exec(command: string, ...args: string): number
// return) exit code
func (jr *JSRuntime) doExec(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		return jr.vm.NewGoError(fmt.Errorf("no command provided"))
	}
	args := []string{call.Arguments[0].String()}
	for i := 1; i < len(call.Arguments); i++ {
		args = append(args, call.Arguments[i].String())
	}
	return jr.exec("", args)
}

func (jr *JSRuntime) exec(source string, args []string) goja.Value {
	env, ok := jr.Env.(global.EnvExec)
	if !ok {
		return jr.vm.NewGoError(fmt.Errorf("environment does not support exec"))
	}
	eb := env.ExecBuilder()
	if eb == nil {
		return jr.vm.NewGoError(fmt.Errorf("no command builder defined"))
	}
	cmd, err := eb(source, args)
	if err != nil {
		return jr.vm.NewGoError(err)
	}

	return jr.exec0(cmd)
}
