package jsh

import (
	_ "embed"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"time"

	"github.com/OutOfBedlam/jsh/log"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
)

type JSRuntime struct {
	Name   string
	Source string
	Args   []string
	Strict bool
	Env    Env

	registry      *require.Registry
	eventLoop     *eventloop.EventLoop
	exitCode      int
	shutdownHooks []func()
	nowFunc       func() time.Time
}

func (jr *JSRuntime) RegisterNativeModule(name string, loader require.ModuleLoader) {
	jr.registry.RegisterNativeModule(name, loader)
}

func (jr *JSRuntime) EventLoop() *eventloop.EventLoop {
	return jr.eventLoop
}

func (jr *JSRuntime) Run() error {
	if jr.Env == nil {
		jr.Env = &DefaultEnv{}
	}

	defer func() {
		if r := recover(); r != nil {
			if ie, ok := r.(*goja.InterruptedError); ok {
				fmt.Fprintf(jr.Env.Writer(), "interrupted: %v\n", ie.Value())
			}
			fmt.Fprintf(os.Stderr, "panic: %v\n%v\n", r, string(debug.Stack()))
			os.Exit(1)
		}
	}()

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
	var retErr error = nil
	jr.eventLoop.Run(func(vm *goja.Runtime) {
		buffer.Enable(vm)
		url.Enable(vm)
		vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
		vm.Set("console", log.SetConsole(vm, jr.Env.Writer()))
		if _, err := vm.RunProgram(program); err != nil {
			retErr = err
			jr.exitCode = -1
		}
	})
	return retErr
}

func (jr *JSRuntime) ExitCode() int {
	return jr.exitCode
}

func (jr *JSRuntime) loadSource(moduleName string) ([]byte, error) {
	return LoadSource(jr.Env, moduleName)
}

func (jr *JSRuntime) AddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

func (jr *JSRuntime) Exec(vm *goja.Runtime, source string, args []string) goja.Value {
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		return vm.NewGoError(fmt.Errorf("no command builder defined"))
	}
	cmd, err := eb(source, args)
	if err != nil {
		return vm.NewGoError(err)
	}
	return jr.exec0(vm, cmd)
}

type Exit struct {
	Code int
}

func (jr *JSRuntime) Module(vm *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)
	exports.Set("env", jr.Env)
	exports.Set("args", jr.Args)
	exports.Set("addShutdownHook", jr.AddShutdownHook)
	exports.Set("exit", doExit(vm))
	exports.Set("exec", doExec(vm, jr.Exec))
	exports.Set("execString", doExecString(vm, jr.Exec))
	exports.Set("dispatchEvent", dispatchEvent(jr.EventLoop()))
	if jr.nowFunc == nil {
		exports.Set("now", time.Now)
	} else {
		exports.Set("now", jr.nowFunc)
	}
}

// doExecString executes a command line string via the exec function.
//
// syntax) execString(source: string, ...args: string): number
// return) exit code
func doExecString(vm *goja.Runtime, exec func(vm *goja.Runtime, source string, args []string) goja.Value) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("no source code provided"))
		}
		args := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			args = append(args, a.String())
		}
		return exec(vm, args[0], args[1:])
	}
}

// doExec executes a command by building an exec.Cmd and running it.
//
// syntax) exec(command: string, ...args: string): number
// return) exit code
func doExec(vm *goja.Runtime, exec func(vm *goja.Runtime, source string, args []string) goja.Value) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("no command provided"))
		}
		args := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			args = append(args, a.String())
		}
		return exec(vm, "", args)
	}
}

func doExit(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		exit := Exit{Code: 0}
		if len(call.Arguments) > 0 {
			exit.Code = int(call.Argument(0).ToInteger())
		}
		vm.Interrupt(exit)
		return goja.Undefined()
	}
}
