package jsh

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

type JSRuntime struct {
	Name        string
	Source      string
	Args        []string
	Strict      bool
	Env         Env
	ExecBuilder ExecBuilderFunc

	program *goja.Program
	vm      *goja.Runtime

	shutdownHooks []func()
	nowFunc       func() time.Time
}

// ExecBuilderFunc is a function that builds an *exec.Cmd given the source and arguments.
// if code is empty, it indicates that the file is being executed from file named in args[0].
// if code is non-empty, it indicates that the code is being executed.
type ExecBuilderFunc func(code string, args []string) (*exec.Cmd, error)

func (jr *JSRuntime) Run() error {
	if jr.Env == nil {
		jr.Env = &DefaultEnv{}
	}

	if program, err := goja.Compile(jr.Name, jr.Source, jr.Strict); err != nil {
		return err
	} else {
		jr.program = program
	}

	if err := jr.initRuntime(); err != nil {
		return err
	}

	if result, err := jr.vm.RunProgram(jr.program); err != nil {
		return err
	} else {
		_ = result
	}

	slices.Reverse(jr.shutdownHooks)
	for _, hook := range jr.shutdownHooks {
		hook()
	}
	return nil
}

func (jr *JSRuntime) initRuntime() error {
	jr.vm = goja.New()
	jr.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	registry := require.NewRegistry(
		require.WithGlobalFolders("node_modules"),
		require.WithLoader(jr.loadSource),
	)
	registry.RegisterNativeModule("@jsh/shell", shell.Module)
	registry.Enable(jr.vm)

	con := jr.vm.NewObject()
	jr.vm.Set("console", con)
	con.Set("log", console_log(jr.Env.Writer(), slog.LevelInfo))
	con.Set("debug", console_log(jr.Env.Writer(), slog.LevelDebug))
	con.Set("info", console_log(jr.Env.Writer(), slog.LevelInfo))
	con.Set("warn", console_log(jr.Env.Writer(), slog.LevelWarn))
	con.Set("error", console_log(jr.Env.Writer(), slog.LevelError))

	obj := jr.vm.NewObject()
	jr.vm.Set("runtime", obj)
	obj.Set("print", jr.Print)
	obj.Set("println", jr.Println)
	obj.Set("now", jr.Now)
	obj.Set("addShutdownHook", jr.AddShutdownHook)
	obj.Set("args", jr.args())
	obj.Set("exec", jr.Exec)
	obj.Set("execString", jr.ExecString)
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

func console_log(w io.Writer, level slog.Level) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments)+1)
		args[0] = level.String()
		args[0] = args[0].(string) + strings.Repeat(" ", 5-len(args[0].(string)))
		for i, arg := range call.Arguments {
			args[i+1] = valueToPrintable(arg)
		}
		fmt.Fprintln(w, args...)
		return goja.Undefined()
	}
}

func (jr *JSRuntime) AddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

func (jr *JSRuntime) Now() goja.Value {
	now := jr.nowFunc
	if now == nil {
		now = time.Now
	}
	return jr.vm.ToValue(now())
}

func (jr *JSRuntime) Print(call goja.FunctionCall) goja.Value {
	args := make([]interface{}, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = valueToPrintable(arg)
	}
	fmt.Fprint(jr.Env.Writer(), args...)
	return goja.Undefined()
}

func (jr *JSRuntime) Println(call goja.FunctionCall) goja.Value {
	call.Arguments = append(call.Arguments, jr.vm.ToValue("\n"))
	return jr.Print(call)
}

func (jr *JSRuntime) args() goja.Value {
	s := make([]any, len(jr.Args))
	for i, arg := range jr.Args {
		s[i] = arg
	}
	return jr.vm.NewArray(s...)
}

func (jr *JSRuntime) ExecString(call goja.FunctionCall) goja.Value {
	args := make([]string, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = arg.String()
	}
	if jr.ExecBuilder == nil {
		panic(jr.vm.ToValue("runtime.exec: no command builder defined"))
	}
	ex, err := jr.ExecBuilder(args[0], args[1:])
	if err != nil {
		panic(jr.vm.ToValue(err.Error()))
	}
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()
	if err := ex.Run(); err != nil {
		panic(jr.vm.ToValue("runtime.exec: " + err.Error()))
	}
	return jr.vm.ToValue(0)
}

func (jr *JSRuntime) Exec(call goja.FunctionCall) goja.Value {
	args := make([]string, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = arg.String()
	}
	if jr.ExecBuilder == nil {
		panic(jr.vm.ToValue("runtime.exec: no command builder defined"))
	}
	ex, err := jr.ExecBuilder("", args)
	if err != nil {
		panic(jr.vm.ToValue(err.Error()))
	}
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()
	if err := ex.Run(); err != nil {
		panic(jr.vm.ToValue("runtime.exec: " + err.Error()))
	}
	return jr.vm.ToValue(0)
}
