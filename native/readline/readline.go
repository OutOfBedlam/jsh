package readline

import (
	"context"
	"io"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	o.Set("Reader", newReader(rt))
}

func newReader(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		conf := DefaultConfig()
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], conf); err != nil {
				panic(rt.NewGoError(err))
			}
		}
		reader := &Reader{
			rt:   rt,
			obj:  rt.NewObject(),
			conf: conf,
		}
		reader.obj.Set("readLine", reader.ReadLine)
		return reader.obj
	}
}

type Config struct {
	Prompt string `json:"prompt"`
}

func DefaultConfig() *Config {
	return &Config{
		Prompt: "> ",
	}
}

type Reader struct {
	rt   *goja.Runtime
	obj  *goja.Object
	conf *Config
	ed   *multiline.Editor
}

func (r *Reader) ReadLine(call goja.FunctionCall) goja.Value {
	if r.ed == nil {
		r.ed = &multiline.Editor{}
		r.ed.SetPrompt(func(w io.Writer, i int) (int, error) {
			if i == 0 {
				return w.Write([]byte(r.conf.Prompt))
			} else {
				return w.Write([]byte(strings.Repeat(" ", len(r.conf.Prompt))))
			}
		})
		r.ed.SubmitOnEnterWhen(func(s []string, i int) bool { return true })
	}
	ctx := context.Background()
	if lines, err := r.ed.Read(ctx); err != nil {
		return r.rt.NewGoError(err)
	} else {
		return r.rt.ToValue(strings.Join(lines, "\n"))
	}
}
