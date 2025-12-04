package jsh

import (
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/OutOfBedlam/jsh/native/log"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

type JSRuntime struct {
	Name   string
	Source string
	Args   []string
	Strict bool
	Env    Env

	vm            *goja.Runtime
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
	registry.RegisterNativeModule("@jsh/shell", shell.Module)
	registry.Enable(jr.vm)

	jr.vm.Set("console", log.SetConsole(jr.vm, jr.Env.Writer()))

	obj := jr.vm.NewObject()
	jr.vm.Set("runtime", obj)
	obj.Set("now", jr.doNow) // TODO: move to external command (.js)
	obj.Set("addShutdownHook", jr.doAddShutdownHook)
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

	if result, err := jr.vm.RunProgram(program); err != nil {
		return err
	} else {
		_ = result
	}

	return nil
}

func (jr *JSRuntime) loadSource(moduleName string) ([]byte, error) {
	fs := jr.Env.Filesystem()
	if fs == nil {
		return nil, fmt.Errorf("no filesystem available to load module: %s", moduleName)
	}
	file, err := fs.Open(moduleName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func (jr *JSRuntime) doAddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
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

func (jr *JSRuntime) doExecString(call goja.FunctionCall) goja.Value {
	args := make([]string, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = arg.String()
	}
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		panic(jr.vm.ToValue("runtime.execString: no command builder defined"))
	}
	ex, err := eb(args[0], args[1:])
	if err != nil {
		panic(jr.vm.ToValue(err.Error()))
	}
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()
	if err := ex.Run(); err != nil {
		panic(jr.vm.ToValue("runtime.execString: " + err.Error()))
	}
	return jr.vm.ToValue(ex.ProcessState.ExitCode())
}

func (jr *JSRuntime) doExec(call goja.FunctionCall) goja.Value {
	args := make([]string, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = arg.String()
	}
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		panic(jr.vm.ToValue("runtime.exec: no command builder defined"))
	}
	ex, err := eb("", args)
	if err != nil {
		panic(jr.vm.ToValue(err.Error()))
	}
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()
	if err := ex.Run(); err != nil {
		panic(jr.vm.ToValue("runtime.exec: " + err.Error()))
	}
	return jr.vm.ToValue(ex.ProcessState.ExitCode())
}
