package global

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
}

type EnvExec interface {
	Env
	ExecBuilder() ExecBuilderFunc
}

// ExecBuilderFunc is a function that builds an *exec.Cmd given the source and arguments.
// if code is empty, it indicates that the file is being executed from file named in args[0].
// if code is non-empty, it indicates that the code is being executed.
type ExecBuilderFunc func(code string, args []string) (*exec.Cmd, error)

type EnvFS interface {
	Env
	Filesystem() fs.FS
}

type EnvNativeModule interface {
	Env
	NativeModules() map[string]require.ModuleLoader
}

func LoadSource(env Env, moduleName string) ([]byte, error) {
	var fileSystem fs.FS
	if efs, ok := env.(EnvFS); ok {
		fileSystem = efs.Filesystem()
	} else {
		return nil, fmt.Errorf("module not found: %s", moduleName)
	}
	if fileSystem == nil {
		return nil, fmt.Errorf("no filesystem available to load module: %s", moduleName)
	}

	var paths []string
	if v := env.Get("PATH"); v == nil {
		paths = []string{""}
	} else {
		paths = strings.Split(v.(string), ":")
	}
	for _, p := range paths {
		path := filepath.Join(p, moduleName)
		path = CleanPath(path)
		b, err := loadSource(fileSystem, path)
		if err == nil {
			return b, nil
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
