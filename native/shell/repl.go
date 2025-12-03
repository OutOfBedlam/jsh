package shell

import (
	"context"
	"io"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/mattn/go-colorable"
)

type Repl struct {
	rt *goja.Runtime
}

func repl(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		repl := &Repl{
			rt: rt,
		}
		obj := rt.NewObject()
		obj.Set("loop", repl.Loop)
		return obj
	}
}

func (sh *Repl) Loop(call goja.FunctionCall) goja.Value {
	var ed multiline.Editor
	ed.SetPrompt(sh.prompt)
	ed.SubmitOnEnterWhen(sh.submitOnEnterWhen)
	ed.SetWriter(colorable.NewColorableStdout())
	ctx := context.Background()
	for {
		var content string
		if lines, err := ed.Read(ctx); err != nil {
			break
		} else {
			if len(lines) == 1 {
				line := strings.TrimSpace(strings.TrimSuffix(lines[0], ";"))
				if line == "exit" || line == "quit" {
					return sh.rt.ToValue(0)
				}
			}
			content = strings.Join(lines, "\n")
		}
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
	return sh.rt.ToValue(0)
}

func (sh *Repl) prompt(w io.Writer, lineNo int) (int, error) {
	if lineNo == 0 {
		return w.Write([]byte("repl> "))
	} else {
		return w.Write([]byte("....  "))
	}
}

func (sh *Repl) submitOnEnterWhen(lines []string, lineNo int) bool {
	if strings.HasSuffix(lines[len(lines)-1], `;`) {
		return true
	}
	return false
}

func (sh *Repl) println(vals ...goja.Value) {
	console := sh.rt.Get("runtime").(*goja.Object)
	print, _ := goja.AssertFunction(console.Get("println"))
	print(goja.Undefined(), vals...)
}
