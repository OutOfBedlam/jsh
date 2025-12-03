package shell

import (
	"context"
	"io"
	"strings"

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
		obj := rt.NewObject()
		obj.Set("run", shell.Run)
		return obj
	}
}

type Shell struct {
	rt *goja.Runtime
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
			if !sh.process(content) {
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

func (sh *Shell) println(vals ...goja.Value) {
	obj := sh.rt.Get("runtime").(*goja.Object)
	print, _ := goja.AssertFunction(obj.Get("println"))
	print(goja.Undefined(), vals...)
}

func (sh *Shell) execString(source string, args []string) {
	obj := sh.rt.Get("runtime").(*goja.Object)
	execString, _ := goja.AssertFunction(obj.Get("execString"))
	execString(goja.Undefined(), sh.rt.ToValue(source), sh.rt.ToValue(args))
}

func (sh *Shell) process(content string) bool {
	// parse content for shell commands
	// shell commands are similar syntactically to unix shell commands
	// it may contains:
	// - pipes using |
	// - re-directions using >, >>, <
	// - continuation using ;
	// - conditional execution using &&

	switch strings.TrimSpace(content) {
	case "exit", "quit":
		return false
	case "repl":
		sh.execString(`const m = require("@jsh/shell"); const r = new m.Repl(); r.loop();`, []string{})
	default:
		val, err := sh.rt.RunString(content)
		if err != nil {
			sh.println(sh.rt.NewGoError(err))
		} else {
			if val != nil && val != goja.Null() && val != goja.Undefined() {
				sh.println(val)
			} else {
				sh.println()
			}
		}
	}
	return true
}
