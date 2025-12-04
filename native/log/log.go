package log

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/dop251/goja"
)

var defaultWriter io.Writer = io.Discard

func SetConsole(vm *goja.Runtime, w io.Writer) *goja.Object {
	defaultWriter = w

	con := vm.NewObject()
	con.Set("log", makeConsoleLog(slog.LevelInfo))
	con.Set("debug", makeConsoleLog(slog.LevelDebug))
	con.Set("info", makeConsoleLog(slog.LevelInfo))
	con.Set("warn", makeConsoleLog(slog.LevelWarn))
	con.Set("error", makeConsoleLog(slog.LevelError))
	con.Set("println", doPrintln)
	con.Set("print", doPrint)
	return con
}

func Println(args ...interface{}) {
	fmt.Fprintln(defaultWriter, args...)
}

func Print(args ...interface{}) {
	fmt.Fprint(defaultWriter, args...)
}

func Log(level slog.Level, args ...interface{}) {
	strLevel := level.String()
	strLevel = strLevel + strings.Repeat(" ", 5-len(strLevel))
	fmt.Fprintln(defaultWriter, strLevel, fmt.Sprint(args...))
}

func doPrint(call goja.FunctionCall) goja.Value {
	Print(argsValues(call)...)
	return goja.Undefined()
}

func doPrintln(call goja.FunctionCall) goja.Value {
	Println(argsValues(call)...)
	return goja.Undefined()
}

func makeConsoleLog(level slog.Level) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		Log(level, argsValues(call)...)
		return goja.Undefined()
	}
}

func argsValues(call goja.FunctionCall) []interface{} {
	args := make([]interface{}, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = valueToPrintable(arg)
	}
	return args
}

func valueToPrintable(value goja.Value) any {
	val := value.Export()
	if obj, ok := val.(*goja.Object); ok {
		toString, ok := goja.AssertFunction(obj.Get("toString"))
		if ok {
			ret, _ := toString(obj)
			return ret
		}
	}
	if m, ok := val.(map[string]any); ok {
		f := []string{}
		for k, v := range m {
			f = append(f, fmt.Sprintf("%s:%v", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(f, ", "))
	}
	if a, ok := val.([]any); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([]float64); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([][]float64); ok {
		f := []string{}
		for _, vv := range a {
			fv := []string{}
			for _, v := range vv {
				fv = append(fv, fmt.Sprintf("%v", v))
			}
			f = append(f, fmt.Sprintf("[%s]", strings.Join(fv, ", ")))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	return fmt.Sprintf("%v", val)
}
