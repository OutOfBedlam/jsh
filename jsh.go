package jsh

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/OutOfBedlam/jsh/native/ws"
)

func Main() int {
	cmd := flag.String("c", "", "command to execute")
	dir := flag.String("d", ".", "working directory")
	dev := flag.String("dev", "", "use development filesystem")
	flag.Parse()

	fileSystem := NewFS()
	fileSystem.Mount("/", Root(*dev))

	if dfs, err := checkFS(*dir); err != nil {
		fmt.Println("Error setting up filesystem:", err.Error())
		os.Exit(1)
	} else {
		fileSystem.Mount("/work", dfs)
	}

	env := NewEnv(
		WithFilesystem(fileSystem),
		WithReader(os.Stdin),
		WithWriter(os.Stdout),
		WithExecBuilder(execBuilder(*dir, *dev)),
		WithNativeModule("@jsh/shell", shell.Module),
		WithNativeModule("@jsh/ws", ws.Module),
	)
	env.Set("PATH", "/work:/sbin")
	env.Set("PWD", "/work")

	args, passthrough := argAndPassthrough(flag.Args())

	scriptName, script := mainScript(env, *cmd, args)
	if script == "" {
		fmt.Println("Command not found: " + args[0])
		return 1
	}

	jr := &JSRuntime{
		Name:   scriptName,
		Source: script,
		Args:   passthrough,
		Env:    env,
	}
	if err := jr.Run(); err != nil {
		fmt.Println("JavaScript runtime error: ", err)
		return 1
	}
	return jr.ExitCode()
}

// mainScript determines the main script to run based on command line inputs.
func mainScript(env global.Env, cmd string, args []string) (scriptName string, script string) {
	if cmd != "" {
		scriptName = "command-line"
		script = cmd
	} else if len(args) > 0 {
		b, err := global.LoadSource(env, args[0])
		if err != nil {
			return
		}
		// replace shebang line as javascript comment
		if b[0] == '#' && b[1] == '!' {
			b[0], b[1] = '/', '/'
		}
		scriptName = args[0]
		script = string(b)
	} else {
		// No command or script file provided
		// start shell
		scriptName = "shell"
		script = defaultShell
	}
	return
}

const defaultShell = `// default user shell
const m = require("@jsh/shell");
const r = new m.Shell();
r.run();
`

//go:embed root/*
var rootFS embed.FS

// Root returns the root filesystem.
func Root(devDir string) fs.FS {
	if devDir != "" {
		dirFS := os.DirFS(devDir)
		return dirFS
	} else {
		rootFS, _ := fs.Sub(rootFS, "root")
		return rootFS
	}
}

// execBuilder builds an exec.Cmd to run jsh with the given code and args.
func execBuilder(dir string, devDir string) global.ExecBuilderFunc {
	return func(code string, args []string) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		opts := []string{}
		if devDir != "" {
			opts = append(opts, "-dev", devDir)
		}
		if code != "" {
			opts = append(opts, "-c", code, "-d", dir)
			if len(args) > 0 {
				opts = append(opts, "--")
				opts = append(opts, args...)
			}
		} else {
			opts = append(opts, "-d", dir, args[0])
			if args := args[1:]; len(args) > 0 {
				opts = append(opts, "--")
				opts = append(opts, args...)
			}
		}
		return exec.Command(self, opts...), nil
	}
}

// argAndPassthrough splits args into those before "--" and those after.
func argAndPassthrough(args []string) (remains []string, passthrough []string) {
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				passthrough = args[i+1:]
			}
			remains = args[:i]
			return
		}
	}
	remains = args
	return
}

// checkFS checks that the given directory exists and is a directory, returning an fs.FS for it.
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
