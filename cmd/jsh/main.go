package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/OutOfBedlam/jsh"
)

func main() {
	cmd := flag.String("c", "", "command to execute")
	dir := flag.String("d", ".", "working directory")
	flag.Parse()

	args, passthrough := argAndPassthrough(flag.Args())
	fileSystem, err := checkFS(*dir)
	if err != nil {
		fmt.Println("Error setting up filesystem:", err.Error())
		os.Exit(1)
	}

	env := jsh.NewEnv(
		jsh.WithFilesystem(fileSystem),
		jsh.WithReader(os.Stdin),
		jsh.WithWriter(os.Stdout),
		jsh.WithExecBuilder(execBuilder(*dir)),
	)

	var script string
	var scriptName string

	if *cmd != "" {
		scriptName = "command-line"
		script = *cmd
	} else if len(args) > 0 {
		filename := args[0]
		if !strings.HasSuffix(filename, ".js") {
			filename += ".js"
		}
		if f, err := fileSystem.Open(filename); err != nil {
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
		Env:    env,
	}
	if err := jr.Run(); err != nil {
		slog.Error("JavaScript runtime error", "error", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func execBuilder(dir string) jsh.ExecBuilderFunc {
	return func(code string, args []string) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		if code != "" {
			args = append([]string{
				"-d", dir,
				"-c", code,
				"--"}, args...)
		} else {
			args = append([]string{
				"-d", dir,
				args[0],
				"--"}, args[1:]...)
		}
		return exec.Command(self, args...), nil
	}
}

func argAndPassthrough(args []string) (remains []string, passthrough []string) {
	for i, arg := range args {
		if arg == "--" {
			passthrough = args[i+1:]
			remains = args[:i]
			return
		}
	}
	remains = args
	return
}

func checkFS(dir string) (fileSystem fs.FS, err error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %v", err)
		}
		return os.DirFS(wd), nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stating directory %q: %v", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %q", dir)
	}
	absDir, err := os.Readlink(dir)
	if err != nil {
		absDir = dir
	}
	return os.DirFS(absDir), nil
}
