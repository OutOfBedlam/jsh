package engine

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
)

func New(conf Config) (*JSRuntime, error) {
	fileSystem := NewFS()
	for _, tab := range conf.FSTabs {
		if dirfs, err := DirFS(tab.Source); err != nil {
			return nil, fmt.Errorf("error mounting %s to %s: %v", tab.Source, tab.MountPoint, err)
		} else {
			fileSystem.Mount(tab.MountPoint, dirfs)
		}
	}
	if fileSystem.mounts["/"] == nil {
		fileSystem.Mount("/", Root(""))
	}

	var reader io.Reader = os.Stdin
	if conf.Reader != nil {
		reader = conf.Reader
	}
	var writer io.Writer = os.Stdout
	if conf.Writer != nil {
		writer = conf.Writer
	}
	var execBuilderFunc ExecBuilderFunc
	if conf.ExecBuilder != nil {
		execBuilderFunc = conf.ExecBuilder
	} else {
		execBuilderFunc = execBuilder(conf.FSTabs)
	}
	opts := []EnvOption{
		WithFilesystem(fileSystem),
		WithReader(reader),
		WithWriter(writer),
		WithExecBuilder(execBuilderFunc),
	}
	env := NewEnv(opts...)
	for k, v := range conf.Env {
		env.Set(k, v)
	}
	// Default environment variables
	if env.Get("PATH") == nil {
		env.Set("PATH", "/sbin:/lib")
	}
	if env.Get("HOME") == nil {
		env.Set("HOME", "/")
	}
	if env.Get("PWD") == nil {
		env.Set("PWD", "/")
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
			// start default command
			b, _ := LoadSource(env, conf.Default)
			scriptName = conf.Default
			script = string(b)
		} else {
			if !strings.HasSuffix(cmd, ".js") {
				cmd = cmd + ".js"
			}
			b, err := LoadSource(env, cmd)
			if err != nil {
				return nil, fmt.Errorf("command not found: %s", cmd)
			}
			// replace shebang line as javascript comment
			if b[0] == '#' && b[1] == '!' {
				b[0], b[1] = '/', '/'
			}
			scriptName = cmd
			script = string(b)
		}
	} else {
		scriptName = conf.Name
		script = conf.Code
		scriptArgs = conf.Args
	}
	if scriptName == "" {
		scriptName = "ad-hoc"
	}

	jr := &JSRuntime{
		Name:   scriptName,
		Source: script,
		Args:   scriptArgs,
		Env:    env,
	}

	jr.registry = require.NewRegistry(
		require.WithLoader(jr.loadSource),
		require.WithPathResolver(jr.pathResolver),
	)
	jr.eventLoop = NewEventLoop(
		eventloop.EnableConsole(false),
		eventloop.WithRegistry(jr.registry),
	)
	return jr, nil
}

func (jr *JSRuntime) Main() int {
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
		fmt.Println("runtime error:", err)
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
func execBuilder(fstabs []FSTab) ExecBuilderFunc {
	useSecretBox := os.Getenv("JSH_NO_SECRET_BOX") != "1"
	return func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		// code and env may contains sensitive information,
		// so use secret box to pass it to the child process.
		if useSecretBox {
			conf := Config{
				Code:   code,
				Args:   args,
				FSTabs: fstabs,
				Env:    env,
			}
			secretBox, err := NewSecretBox(conf)
			if err != nil {
				return nil, err
			}
			execCmd := exec.Command(self, "-s", secretBox.FilePath(), args[0])
			return execCmd, nil
		} else {
			opts := []string{}
			for _, tab := range fstabs {
				opts = append(opts, "-v", fmt.Sprintf("%s=%s", tab.MountPoint, tab.Source))
			}
			if code != "" {
				opts = append(opts, "-c", code)
				if len(args) > 0 {
					opts = append(opts, args...)
				}
			} else {
				opts = append(opts, args[0])
				if args := args[1:]; len(args) > 0 {
					opts = append(opts, args...)
				}
			}
			return exec.Command(self, opts...), nil
		}
	}
}

// DirFS checks that the given directory exists and is a directory, returning an fs.FS for it.
func DirFS(dir string) (fileSystem fs.FS, err error) {
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
	Name   string         `json:"name"`
	Code   string         `json:"code"`
	Args   []string       `json:"args"`
	Env    map[string]any `json:"env"`
	FSTabs []FSTab        `json:"fstabs,omitempty"`

	Default     string          `json:"default,omitempty"`
	Writer      io.Writer       `json:"-"`
	Reader      io.Reader       `json:"-"`
	ExecBuilder ExecBuilderFunc `json:"-"`
}

type FSTab struct {
	MountPoint string `json:"mountPoint"`
	Source     string `json:"source"`
	Type       string `json:"type,omitempty"`
	Options    string `json:"options,omitempty"`
}

type FSTabs []FSTab

func (m *FSTabs) String() string {
	b, err := json.Marshal(m)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// Set(stirng) error is required to implement flag.Value interface.
// Set parses and adds a new FSTab from the given string.
// The format is /mountpoint=source
func (m *FSTabs) Set(value string) error {
	tokens := strings.SplitN(value, "=", 2)
	if len(tokens) != 2 {
		return fmt.Errorf("invalid mount option: %s", value)
	}
	*m = append(*m, FSTab{
		MountPoint: tokens[0],
		Source:     tokens[1],
	})
	return nil
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
