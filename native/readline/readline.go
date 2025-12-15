package readline

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/nyaosorg/go-ttyadapter/auto"
)

//go:embed readline.js
var readlineJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	module.Set("NewReadLine", NewReadLine(rt))

	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString(fmt.Sprintf(`(()=>{%s})()`, readlineJS))
	if err != nil {
		panic(err)
	}
}

func NewReadLine(vm *goja.Runtime) func(obj *goja.Object, opt *goja.Object) *Reader {
	return func(obj *goja.Object, opt *goja.Object) *Reader {
		reader := &Reader{
			vm: vm,
			ed: &multiline.Editor{},
		}
		return reader
	}
}

type Reader struct {
	vm     *goja.Runtime
	ed     *multiline.Editor
	cancel context.CancelFunc
}

type Options struct {
	AutoInput         []string
	Prompt            goja.Callable
	SubmitOnEnterWhen goja.Callable
}

func (r *Reader) ReadLine(conf Options) (string, error) {
	// AutoInput / AutoOutput
	if len(conf.AutoInput) > 0 {
		r.ed.LineEditor.Tty = &auto.Pilot{
			Text: conf.AutoInput,
		}
		r.ed.SetWriter(io.Discard)
	}

	// Prompt
	r.ed.SetPrompt(func(w io.Writer, line int) (int, error) {
		prompt := "> "
		if conf.Prompt == nil {
			if line == 0 {
				return w.Write([]byte(prompt))
			} else {
				return w.Write(append([]byte(strings.Repeat(".", len(prompt)-1)), ' '))
			}
		}
		p, _ := conf.Prompt(goja.Undefined(), r.vm.ToValue(line))
		prompt = fmt.Sprintf("%v", p.Export())
		return w.Write([]byte(prompt))
	})
	// SubmitOnEnterWhen
	r.ed.SubmitOnEnterWhen(func(s []string, idx int) bool {
		if conf.SubmitOnEnterWhen == nil {
			return true
		}
		result := false
		b, err := conf.SubmitOnEnterWhen(goja.Undefined(), r.vm.ToValue(s), r.vm.ToValue(idx))
		if err != nil {
			fmt.Println("SubmitOnEnterWhen error:", err)
			return false
		}
		result = b.Export().(bool)
		return result
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.cancel = cancel

	if lines, err := r.ed.Read(ctx); err != nil {
		return "", err
	} else {
		return strings.Join(lines, "\n"), nil
	}
}

func (r *Reader) Close() {
	if cancel := r.cancel; cancel != nil {
		cancel()
	}
}
