package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

	"github.com/OutOfBedlam/jsh"
)

func main() {
	cmd := flag.String("c", "", "command to execute")
	dir := flag.String("d", ".", "working directory")
	flag.Parse()

	var passthrough []string
	var args = flag.Args()
	for i, arg := range args {
		if arg == "--" {
			passthrough = args[i+1:]
			args = args[:i]
			break
		}
	}

	var fsRoot string = "."
	if dir != nil && *dir != "" {
		fsRoot = *dir
	}
	var fs = os.DirFS(fsRoot)

	var script string
	var scriptName string

	if *cmd != "" {
		scriptName = "command-line"
		script = *cmd
	} else if args := flag.Args(); len(args) > 0 {
		filename := args[0]
		if f, err := fs.Open(filename); err != nil {
			fmt.Println("Error opening script file:", err.Error())
			os.Exit(1)
		} else {
			defer f.Close()
			sc, err := io.ReadAll(f)
			if err != nil {
				fmt.Println("Error reading script file:", err.Error())
				os.Exit(1)
			}
			scriptName = filename
			script = string(sc)
		}
	} else {
		// No command or script file provided
		// start shell
		scriptName = "shell"
		script = `const m = require("@jsh/shell"); const r = new m.Shell(); r.run();`
	}

	jr := &jsh.JSRuntime{
		Name:   scriptName,
		Source: script,
		Args:   passthrough,
		Env: jsh.NewEnv(
			jsh.WithFilesystem(fs),
			jsh.WithReader(os.Stdin),
			jsh.WithWriter(os.Stdout),
		),
		ExecBuilder: func(code string, args []string) (*exec.Cmd, error) {
			self, err := os.Executable()
			if err != nil {
				return nil, err
			}
			if code != "" {
				args = append([]string{
					"-d", *dir,
					"-c", code,
					"--"}, args...)
			} else {
				args = append([]string{
					"-d", *dir,
					args[0],
					"--"}, args[1:]...)
			}
			return exec.Command(self, args...), nil
		},
	}
	if err := jr.Run(); err != nil {
		slog.Error("JavaScript runtime error", "error", err)
		os.Exit(1)
	}
	os.Exit(0)
}
