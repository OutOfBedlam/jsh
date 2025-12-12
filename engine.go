package jsh

import (
	_ "embed"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"time"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/OutOfBedlam/jsh/log"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
)

type JSRuntime struct {
	Name   string
	Source string
	Args   []string
	Strict bool
	Env    global.Env

	exitCode      int
	shutdownHooks []func()
	nowFunc       func() time.Time
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

	registry := require.NewRegistry(
		require.WithGlobalFolders("node_modules"),
		require.WithLoader(jr.loadSource),
	)
	if envNM, ok := jr.Env.(global.EnvNativeModule); ok {
		for name, loader := range envNM.NativeModules() {
			registry.RegisterNativeModule(name, loader)
		}
	}

	loop := global.NewEventLoop(
		eventloop.EnableConsole(false),
		eventloop.WithRegistry(registry),
	)

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
	loop.Run(func(vm *goja.Runtime) {
		vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
		vm.Set("console", log.SetConsole(vm, jr.Env.Writer()))
		vm.Set("eventloop", loop)
		if jr.nowFunc == nil {
			vm.Set("now", time.Now)
		} else {
			vm.Set("now", jr.nowFunc)
		}

		obj := vm.NewObject()
		vm.Set("runtime", obj)
		obj.Set("env", jr.Env)
		obj.Set("args", jr.Args)
		obj.Set("addShutdownHook", jr.doAddShutdownHook)
		obj.Set("exit", doExit(vm))
		obj.Set("exec", doExec(vm, jr.exec))
		obj.Set("execString", doExecString(vm, jr.exec))

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
	return global.LoadSource(jr.Env, moduleName)
}

func (jr *JSRuntime) doAddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

type Exit struct {
	Code int
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

func (jr *JSRuntime) exec(vm *goja.Runtime, source string, args []string) goja.Value {
	env, ok := jr.Env.(global.EnvExec)
	if !ok {
		return vm.NewGoError(fmt.Errorf("environment does not support exec"))
	}
	eb := env.ExecBuilder()
	if eb == nil {
		return vm.NewGoError(fmt.Errorf("no command builder defined"))
	}
	cmd, err := eb(source, args)
	if err != nil {
		return vm.NewGoError(err)
	}
	return jr.exec0(vm, cmd)
}
