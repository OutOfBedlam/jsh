package shell

import (
	"context"
	"io"
	"strings"

	"github.com/OutOfBedlam/jsh/native/log"
	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/mattn/go-colorable"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	// shell = new Shell()
	o.Set("Shell", shell(rt))
	o.Set("Repl", repl(rt))
}

func shell(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		shell := &Shell{
			rt: rt,
		}

		if shell.cwd == "" {
			shell.cwd = "/"
		}
		obj := rt.NewObject()
		obj.Set("run", shell.Run)
		return obj
	}
}

type Shell struct {
	rt  *goja.Runtime
	cwd string
}

func (sh *Shell) Run(call goja.FunctionCall) goja.Value {
	var ed multiline.Editor
	ed.SetPrompt(sh.prompt)
	ed.SubmitOnEnterWhen(sh.submitOnEnterWhen)
	ed.SetWriter(colorable.NewColorableStdout())
	ctx := context.Background()
	for {
		var content string
		if lines, err := ed.Read(ctx); err != nil {
			panic(err)
		} else {
			for i, line := range lines {
				lines[i] = strings.TrimSuffix(line, `\`)
			}
			content = strings.Join(lines, "\n")
			if _, alive := sh.process(content); !alive {
				return sh.rt.ToValue(0)
			}
		}
	}
}

func (sh *Shell) prompt(w io.Writer, lineNo int) (int, error) {
	if lineNo == 0 {
		return w.Write([]byte("jsh> "))
	} else {
		return w.Write([]byte("...  "))
	}
}

func (sh *Shell) submitOnEnterWhen(lines []string, i int) bool {
	if strings.HasSuffix(lines[len(lines)-1], `\`) {
		return false
	}
	return true
}

func (sh *Shell) execString(source string, args []string) goja.Value {
	obj := sh.rt.Get("runtime").(*goja.Object)
	execString, _ := goja.AssertFunction(obj.Get("execString"))
	val, _ := execString(goja.Undefined(), sh.rt.ToValue(source), sh.rt.ToValue(args))
	return val
}

func (sh *Shell) exec(source string, args []string) goja.Value {
	obj := sh.rt.Get("runtime").(*goja.Object)
	exec, _ := goja.AssertFunction(obj.Get("exec"))
	val, _ := exec(goja.Undefined(), sh.rt.ToValue(source), sh.rt.ToValue(args))
	return val
}

// if return false, exit shell
func (sh *Shell) process(content string) (int, bool) {
	// Parse the command
	cmd := parseCommand(content)

	for _, stmt := range cmd.Statements {
		var stopOnError bool
		if stmt.Operator == "&&" {
			stopOnError = true
		}
		for _, pipe := range stmt.Pipelines {
			switch pipe.Command {
			case "exit", "quit":
				return 0, false
			case "repl":
				r := sh.doRepl(pipe.Args)
				if r != 0 && stopOnError {
					return r, true
				}
			case "pwd":
				sh.doPwd()
			default:
				cmd := pipe.Command
				if !strings.HasSuffix(cmd, ".js") {
					cmd += ".js"
				}
				val := sh.exec(cmd, pipe.Args)
				ret := val.Export().(int64)
				if ret != 0 && stopOnError {
					return int(ret), true
				}
			}
		}
	}
	return 0, true
}

func (sh *Shell) doRepl(args []string) int {
	val := sh.execString(`const m = require("@jsh/shell"); const r = new m.Repl(); r.loop();`, args)
	ret := val.Export().(int64)
	return int(ret)
}

func (sh *Shell) doPwd() goja.Value {
	log.Println(sh.cwd)
	return sh.rt.ToValue(sh.cwd)
}

// func (jr *JSRuntime) doCd(call goja.FunctionCall) goja.Value {
// 	if len(call.Arguments) < 1 {
// 		return jr.vm.NewGoError(fmt.Errorf("runtime.cd: missing argument"))
// 	}
// 	dir := call.Arguments[0].String()
// 	if !strings.HasPrefix(dir, "/") {
// 		dir = filepath.Clean(filepath.Join(jr.cwd, dir))
// 	}
// 	nfo, err := fs.Stat(jr.Env.Filesystem(), dir[1:])
// 	if err != nil {
// 		return jr.vm.NewGoError(fmt.Errorf("runtime.cd: %v", err.Error()))
// 	}
// 	if !nfo.IsDir() {
// 		return jr.vm.NewGoError(fmt.Errorf("runtime.cd: not a directory: %q", dir))
// 	}
// 	jr.cwd = dir
// 	return jr.vm.ToValue(jr.cwd)
// }
