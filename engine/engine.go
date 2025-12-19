package engine

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

func (jr *JSRuntime) pathResolver(base, target string) string {
	return require.DefaultPathResolver(base, target)
}

func (jr *JSRuntime) AddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

func (jr *JSRuntime) Exec(vm *goja.Runtime, source string, args []string) goja.Value {
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		return vm.NewGoError(fmt.Errorf("no command builder defined"))
	}
	var env map[string]any
	if de, ok := jr.Env.(*DefaultEnv); ok {
		env = de.vars
	}
	cmd, err := eb(source, args, env)
	if err != nil {
		return vm.NewGoError(err)
	}
	return jr.exec0(vm, cmd)
}
