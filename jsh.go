package jsh

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OutOfBedlam/jsh/global"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

func Run(conf Config) int {
	fileSystem := NewFS()
	fileSystem.Mount("/", Root(conf.Dev))

	if dfs, err := checkFS(conf.Dir); err != nil {
		fmt.Println("Error setting up filesystem:", err.Error())
		os.Exit(1)
	} else {
		fileSystem.Mount("/work", dfs)
	}

	opts := []EnvOption{
		WithFilesystem(fileSystem),
		WithReader(os.Stdin),
		WithWriter(os.Stdout),
		WithExecBuilder(execBuilder(conf.Dir, conf.Dev)),
	}
	for n, m := range conf.ExtNativeModules {
		opts = append(opts, WithNativeModule(n, m))
	}
	env := NewEnv(opts...)
	env.Set("PATH", "/work:/sbin")
	env.Set("PWD", "/work")
	for k, v := range conf.Env {
		env.Set(k, v)
	}

	script := ""
	scriptName := ""
	scriptArgs := []string{}
	if conf.Code == "" {
		cmd := ""
		if len(conf.Args) > 0 {
			cmd = conf.Args[0]
		}
		if len(conf.Args) > 1 {
			scriptArgs = conf.Args[1:]
		}
		if cmd == "" {
			// No command or script file provided
			// start shell
			b, _ := global.LoadSource(env, "/sbin/shell.js")
			scriptName = "shell.js"
			script = string(b)
		} else {
			b, err := global.LoadSource(env, cmd)
			if err != nil {
				fmt.Println("Command not found: " + cmd)
				return 1
			}
			// replace shebang line as javascript comment
			if b[0] == '#' && b[1] == '!' {
				b[0], b[1] = '/', '/'
			}
			scriptName = cmd
			script = string(b)
		}
	} else {
		scriptName = "command-line"
		script = conf.Code
		scriptArgs = conf.Args
	}

	jr := &JSRuntime{
		Name:   scriptName,
		Source: script,
		Args:   scriptArgs,
		Env:    env,
	}

	if err := jr.Run(); err != nil {
		if ie, ok := err.(*goja.InterruptedError); ok {
			frame := ie.Stack()[0]
			if exit, ok := ie.Value().(Exit); ok {
				if exit.Code < 0 {
					fmt.Printf("exit code %d at %v\n", exit.Code, frame.Position())
				}
				return exit.Code
			}
		}
		fmt.Print("runtime error:", err)
		return 1
	}
	return jr.ExitCode()
}

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
	useSecretBox := os.Getenv("JSH_NO_SECRET_BOX") != "1"
	return func(code string, args []string) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		// code and env may contains sensitive information,
		// so use secret box to pass it to the child process.
		if useSecretBox {
			conf := Config{
				Code: code,
				Args: args,
				Dir:  dir,
				Dev:  devDir,
				Env:  map[string]string{},
			}
			secretBox, err := NewSecretBox(conf)
			if err != nil {
				return nil, err
			}
			execCmd := exec.Command(self, "-s", secretBox.FilePath(), args[0])
			return execCmd, nil
		} else {
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

type Config struct {
	Code string            `json:"code"`
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
	Dir  string            `json:"dir"`
	Dev  string            `json:"dev"`

	ExtNativeModules map[string]require.ModuleLoader `json:"extNativeModules,omitempty"`
}

type SecretBox struct {
	secretFile string
}

func NewSecretBox(secret any) (*SecretBox, error) {
	// gen random file name
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	filename := fmt.Sprintf("jsh-%d-%s", os.Getpid(), hex.EncodeToString(randomBytes))

	secretFile := filepath.Join(os.TempDir(), filename)

	// 0600 owner read/write
	fd, err := os.OpenFile(secretFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	enc := json.NewEncoder(fd)
	if err := enc.Encode(secret); err != nil {
		return nil, err
	}

	return &SecretBox{secretFile: secretFile}, nil
}

func (sb *SecretBox) FilePath() string {
	return sb.secretFile
}

func (sb *SecretBox) Cleanup() {
	if sb.secretFile == "" {
		return
	}
	os.WriteFile(sb.secretFile, []byte(""), 0600)
	os.Remove(sb.secretFile)
}

func ReadSecretBox(secretFile string, o interface{}) error {
	defer func() {
		// delete the file
		os.WriteFile(secretFile, []byte(""), 0600)
		os.Remove(secretFile)
	}()

	fd, err := os.Open(secretFile)
	if err != nil {
		return err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	return dec.Decode(o)
}
