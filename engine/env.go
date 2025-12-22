package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type Env interface {
	Reader() io.Reader
	Writer() io.Writer
	Set(key string, value any)
	Get(key string) any
	ExecBuilder() ExecBuilderFunc
	Filesystem() fs.FS
}

// ExecBuilderFunc is a function that builds an *exec.Cmd given the source and arguments.
// if code is empty, it indicates that the file is being executed from file named in args[0].
// if code is non-empty, it indicates that the code is being executed.
type ExecBuilderFunc func(code string, args []string, env map[string]any) (*exec.Cmd, error)

func PathResolver(env Env, base, path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	// require.DefaultPathResolver(base, target)
	p := filepath.Join(filepath.ToSlash(base), filepath.Base(path))
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.ToSlash(p)
}

func LoadSource(env Env, moduleName string) ([]byte, error) {
	moduleName = filepath.ToSlash(moduleName) // for Windows compatibility
	var fileSystem fs.FS = env.Filesystem()
	if fileSystem == nil {
		return nil, fmt.Errorf("no filesystem available to load module: %s", moduleName)
	}

	if strings.HasPrefix(moduleName, "/") {
		moduleName = CleanPath(moduleName)
		b, err := loadSource(fileSystem, moduleName)
		if err == nil {
			return b, nil
		}
	} else {
		findings := []string{
			moduleName,
		}
		if v := env.Get("PATH"); v != nil {
			for _, p := range strings.Split(v.(string), ":") {
				p = filepath.Join(p, moduleName)
				p = filepath.ToSlash(p)
				findings = append(findings, p)
			}
		}
		for _, path := range findings {
			path = CleanPath(path)
			b, err := loadSource(fileSystem, path)
			if err == nil {
				return b, nil
			}
		}
	}
	return nil, fmt.Errorf("module not found: %s", moduleName)
}

func loadSource(fileSystem fs.FS, moduleName string) ([]byte, error) {
	file, err := fileSystem.Open(moduleName)
	if err != nil {
		if !strings.HasSuffix(moduleName, ".js") {
			file, err = fileSystem.Open(moduleName + ".js")
		}
		if err != nil {
			return nil, err
		}
	}
	defer file.Close()
	isDir := false
	if fi, err := file.Stat(); err != nil {
		return nil, err
	} else if fi.IsDir() {
		isDir = true
	}
	if isDir {
		return loadSourceFromDir(fileSystem, moduleName)
	} else {
		return io.ReadAll(file)
	}
}

func loadSourceFromDir(fileSystem fs.FS, moduleName string) ([]byte, error) {
	// look for package.json
	pkgFile, err := fileSystem.Open(moduleName + "/package.json")
	if err == nil {
		defer pkgFile.Close()
		pkgData, err := io.ReadAll(pkgFile)
		if err != nil {
			return nil, err
		}
		var mainEntry struct {
			Main string `json:"main"`
		}
		if err := json.Unmarshal(pkgData, &mainEntry); err != nil {
			return nil, err
		}
		if mainEntry.Main != "" {
			mainPath := filepath.Join(moduleName, mainEntry.Main)
			mainPath = filepath.ToSlash(mainPath)
			if !strings.HasSuffix(mainPath, ".js") {
				mainPath += ".js"
			}
			if main, err := fileSystem.Open(mainPath); err == nil {
				defer main.Close()
				return io.ReadAll(main)
			}
		}
	} else {
		// look for index.js
		indexPath := moduleName + "/index.js"
		if f, err := fileSystem.Open(indexPath); err == nil {
			defer f.Close()
			return io.ReadAll(f)
		}
	}
	return nil, fs.ErrNotExist
}

// cleanPath normalizes a path and ensures it starts with /
func CleanPath(p string) string {
	if p == "" || p == "/" || p == "." {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	if p == "." {
		return "/"
	}
	return p
}

type DefaultEnv struct {
	writer      io.Writer
	reader      io.Reader
	fs          fs.FS
	execBuilder ExecBuilderFunc
	vars        map[string]any
}

var _ Env = (*DefaultEnv)(nil)

func NewEnv(opts ...EnvOption) Env {
	ret := &DefaultEnv{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type EnvOption func(*DefaultEnv)

func WithFilesystem(fs fs.FS) EnvOption {
	return func(de *DefaultEnv) {
		de.fs = fs
	}
}

func WithWriter(w io.Writer) EnvOption {
	return func(de *DefaultEnv) {
		de.writer = w
	}
}

func WithReader(r io.Reader) EnvOption {
	return func(de *DefaultEnv) {
		de.reader = r
	}
}

func WithExecBuilder(eb ExecBuilderFunc) EnvOption {
	return func(de *DefaultEnv) {
		de.execBuilder = eb
	}
}

func (de *DefaultEnv) Reader() io.Reader {
	if de.reader != nil {
		return de.reader
	}
	return os.Stdin
}

func (de *DefaultEnv) Writer() io.Writer {
	if de.writer != nil {
		return de.writer
	}
	return os.Stdout
}

func (de *DefaultEnv) Filesystem() fs.FS {
	return de.fs
}

func (de *DefaultEnv) ExecBuilder() ExecBuilderFunc {
	return de.execBuilder
}

func (de *DefaultEnv) Set(key string, value any) {
	if de.vars == nil {
		de.vars = make(map[string]any)
	}
	if value == nil {
		delete(de.vars, key)
		return
	}
	de.vars[key] = value
}

func (de *DefaultEnv) Get(key string) any {
	if de.vars == nil {
		return nil
	}
	return de.vars[key]
}
