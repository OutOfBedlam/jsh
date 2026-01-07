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

	"github.com/dop251/goja_nodejs/require"
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

// Which looks for the command in the PATH environment variable and returns the full path to the command.
func Which(env Env, command string) string {
	if !strings.HasSuffix(command, ".js") {
		command += ".js"
	}
	if strings.HasPrefix(command, "/") {
		return command
	}
	filesystem := env.Filesystem()
	pathVar := env.Get("PATH")
	if pathStr, ok := pathVar.(string); ok {
		paths := strings.Split(pathStr, ":")
		for _, dir := range paths {
			fullPath := filepath.Join(dir, command)
			fullPath = ResolvePath(env, fullPath)
			if fi, err := filesystem.Open(fullPath); err == nil {
				fi.Close()
				return fullPath
			}
		}
	}
	return ""
}

// expand $VAR and ${VAR} form environment
func Expand(env Env, str string) string {
	str = os.Expand(str, func(varName string) string {
		val := env.Get(varName)
		if valStr, ok := val.(string); ok {
			return valStr
		}
		return ""
	})
	return str
}

// ResolvePath resolves a path by expanding ~ to HOME and expanding environment variables.
func ResolvePath(env Env, path string) string {
	if strings.HasPrefix(path, "~") {
		home := env.Get("HOME")
		if homeStr, ok := home.(string); ok {
			path = filepath.Join(homeStr, path[1:])
		}
	}
	path = os.Expand(path, func(varName string) string {
		val := env.Get(varName)
		if valStr, ok := val.(string); ok {
			return valStr
		}
		return ""
	})
	path = filepath.Clean(path)
	return filepath.ToSlash(path)
}

func ResolveAbsPath(env Env, path string) string {
	path = ResolvePath(env, path)
	if !strings.HasPrefix(path, "/") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			path = filepath.Join(cwdStr, path)
			path = filepath.ToSlash(path)
		}
	}
	return path
}

func GlobalFolders(env Env) []string {
	path := env.Get("LIBRARY_PATH")
	if pathStr, ok := path.(string); ok && pathStr != "" {
		parts := strings.Split(pathStr, ":")
		return parts
	}
	return []string{}
}

func PathResolver(env Env, base, path string) string {
	if base == "." || strings.HasPrefix(base, "./") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			base = filepath.ToSlash(filepath.Join(cwdStr, base))
		}
	} else if base == ".." || strings.HasPrefix(base, "../") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			base = filepath.ToSlash(filepath.Join(cwdStr, base))
		}
	}
	resolved := require.DefaultPathResolver(base, path)
	resolved = filepath.ToSlash(resolved)

	var filesystem fs.FS = env.Filesystem()
	// resolve as .js file
	asFile := resolved
	if !strings.HasSuffix(asFile, ".js") {
		asFile += ".js"
	}
	if f, err := filesystem.Open(asFile); err == nil {
		f.Close()
		return asFile
	}
	// resolve as directory/index.js
	asIndex := resolved + "/index.js"
	if f, err := filesystem.Open(asIndex); err == nil {
		f.Close()
		return asIndex
	}
	// resolve as directory/package.json main entry
	pkgPath := resolved + "/package.json"
	pkgFile, err := filesystem.Open(pkgPath)
	if err == nil {
		defer pkgFile.Close()
		pkgData, err := io.ReadAll(pkgFile)
		if err == nil {
			var mainEntry struct {
				Main string `json:"main"`
			}
			if err := json.Unmarshal(pkgData, &mainEntry); err == nil {
				if mainEntry.Main != "" {
					mainPath := filepath.Join(resolved, mainEntry.Main)
					mainPath = filepath.ToSlash(mainPath)
					if !strings.HasSuffix(mainPath, ".js") {
						mainPath += ".js"
					}
					if f, err := filesystem.Open(mainPath); err == nil {
						f.Close()
						return mainPath
					}
				}
			}
		}
	}
	return resolved
}

func LoadSource(env Env, moduleName string) ([]byte, error) {
	moduleName = filepath.ToSlash(moduleName) // for Windows compatibility
	var fileSystem fs.FS = env.Filesystem()
	if fileSystem == nil {
		return nil, fmt.Errorf("no filesystem available to load module: %s", moduleName)
	}
	if strings.HasPrefix(moduleName, "/") {
		b, err := loadSource(fileSystem, moduleName)
		if err == nil {
			return b, nil
		}
	}
	return nil, require.ModuleFileDoesNotExistError
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
