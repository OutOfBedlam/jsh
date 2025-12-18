package engine

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dop251/goja"
)

func (jr *JSRuntime) Process(vm *goja.Runtime, module *goja.Object) {
	executable, _ := os.Executable()
	exports := module.Get("exports").(*goja.Object)
	exports.Set("env", jr.Env)
	exports.Set("argv", append([]string{executable, jr.Name}, jr.Args...))
	exports.Set("addShutdownHook", jr.AddShutdownHook)
	exports.Set("exit", doExit(vm))
	exports.Set("exec", doExec(vm, jr.Exec))
	exports.Set("execString", doExecString(vm, jr.Exec))
	exports.Set("dispatchEvent", dispatchEvent(jr.EventLoop()))
	exports.Set("now", jr.Now)
	exports.Set("chdir", jr.Chdir)
	exports.Set("cwd", jr.Cwd)
}

func (jr *JSRuntime) Now() time.Time {
	if jr.nowFunc == nil {
		return time.Now()
	} else {
		return jr.nowFunc()
	}
}

func (jr *JSRuntime) Cwd() string {
	return jr.Env.Get("PWD").(string)
}

func (jr *JSRuntime) Chdir(path string) error {
	// Get target directory
	if path == "" || path == "~" {
		path = jr.Env.Get("HOME").(string)
	}
	pwd := jr.Cwd()

	// Handle relative paths
	if !strings.HasPrefix(path, "/") {
		path = pwd + "/" + path
	}
	path = CleanPath(path)

	// Check if directory exists
	fs := jr.Env.Filesystem()
	fd, err := fs.Open(path)
	if err != nil {
		return fmt.Errorf("chdir: no such file or directory: %s", path)
	}
	defer fd.Close()
	info, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("chdir: cannot stat directory: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("chdir: not a directory: %s", path)
	}
	jr.Env.Set("PWD", path)
	return nil
}

type Exit struct {
	Code int
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
