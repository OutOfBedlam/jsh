package shell

import (
	"context"
	"io"
	"strings"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/OutOfBedlam/jsh/log"
	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/hymkor/go-multiline-ny/completion"
	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
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
			rt:      rt,
			history: NewHistory("history", 100),
		}

		if val := rt.Get("runtime").ToObject(rt).Get("env"); val != nil {
			if env := val.Export().(global.Env); env != nil {
				shell.env = env
			}
		}
		if shell.env.Get("PWD") == nil {
			shell.env.Set("PWD", "/")
		}

		obj := rt.NewObject()
		obj.Set("run", shell.Run)
		return obj
	}
}

type Shell struct {
	rt      *goja.Runtime
	env     global.Env
	pwd     string
	history *History
}

func (sh *Shell) Run(call goja.FunctionCall) goja.Value {
	var ed multiline.Editor
	ed.SetPrompt(sh.prompt)
	ed.SubmitOnEnterWhen(sh.submitOnEnterWhen)
	ed.SetWriter(colorable.NewColorableStdout())
	ed.SetHistory(sh.history)
	ed.SetHistoryCycling(true)
	ed.SetPredictColor([...]string{"\x1B[3;22;30m", "\x1B[23;39m"}) // dark gray, italic
	ed.ResetColor = "\x1B[0m"
	ed.DefaultColor = "\x1B[37;49;1m"

	// enable completion
	ed.BindKey(keys.CtrlI, &completion.CmdCompletionOrList{
		Delimiter:  "&|><",
		Enclosure:  `"'`,
		Postfix:    " ",
		Candidates: sh.getCompletionCandidates,
	})
	ctx := context.Background()
	for {
		var line string
		if input, err := ed.Read(ctx); err != nil {
			if err == readline.CtrlC || err == io.EOF {
				return sh.rt.ToValue(0)
			}
			log.Printf("Error input: %v\n", err)
			return sh.rt.ToValue(1)
		} else {
			sh.history.Add(strings.Join(input, "\n"))
			for i, ln := range input {
				input[i] = strings.TrimSuffix(ln, `\`)
			}
			line = strings.Join(input, "")
		}

		if _, alive := sh.process(line); !alive {
			return sh.rt.ToValue(0)
		}
	}
}

func (sh *Shell) prompt(w io.Writer, lineNo int) (int, error) {
	if lineNo == 0 {
		return w.Write([]byte("\033[31mjsh>\033[0m "))
	} else {
		return w.Write([]byte("\033[31m...\033[0m  "))
	}
}

func (sh *Shell) submitOnEnterWhen(lines []string, i int) bool {
	if strings.HasSuffix(lines[len(lines)-1], `\`) {
		return false
	}
	return true
}

func (sh *Shell) getCompletionCandidates(fields []string) (forCompletion []string, forListing []string) {
	return
}

// if return false, exit shell
func (sh *Shell) process(line string) (int, bool) {
	// Parse the command
	cmd := parseCommand(line)

	for _, stmt := range cmd.Statements {
		var stopOnError bool
		if stmt.Operator == "&&" {
			stopOnError = true
		}
		for _, pipe := range stmt.Pipelines {
			switch pipe.Command {
			case "exit", "quit":
				return 0, false
			default:
				cmd := pipe.Command
				if !strings.HasSuffix(cmd, ".js") {
					cmd += ".js"
				}
				exitCode := -1
				val := sh.exec(cmd, pipe.Args)
				switch v := val.Export().(type) {
				default:
					log.Println(val.String())
				case int64:
					exitCode = int(v)
				}
				if exitCode != 0 && stopOnError {
					return exitCode, true
				}
			}
		}
	}
	return 0, true
}

func (sh *Shell) exec(command string, args []string) goja.Value {
	obj := sh.rt.Get("runtime").(*goja.Object)
	exec, _ := goja.AssertFunction(obj.Get("exec"))
	values := []goja.Value{sh.rt.ToValue(command)}
	for _, arg := range args {
		values = append(values, sh.rt.ToValue(arg))
	}
	val, _ := exec(goja.Undefined(), values...)
	return val
}
