package jsh

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/OutOfBedlam/jsh/global"
)

type TestEnv struct {
	input       bytes.Buffer
	output      bytes.Buffer
	execBuilder global.ExecBuilderFunc
}

var _ global.Env = (*TestEnv)(nil)

func (te *TestEnv) Reader() io.Reader {
	return &te.input
}

func (te *TestEnv) Writer() io.Writer {
	return &te.output
}

func (te *TestEnv) Filesystem() fs.FS {
	fileSystem := NewFS()
	fileSystem.Mount("/work", os.DirFS("./test/"))
	return fileSystem
}

func (te *TestEnv) ExecBuilder() global.ExecBuilderFunc {
	return testExecBuilder
}

func (te *TestEnv) Set(key string, value any) {
	// No-op for test env
}

func (te *TestEnv) Get(key string) any {
	switch key {
	case "PATH":
		return "/:/sbin:/work"
	}
	return nil
}

type TestCase struct {
	name     string
	script   string
	input    []string
	output   []string
	preTest  func(*JSRuntime)
	postTest func(*JSRuntime)
}

func runTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		env := &TestEnv{}
		env.input.WriteString(strings.Join(tc.input, "\n") + "\n")

		jr := &JSRuntime{
			Name:   tc.name,
			Source: tc.script,
			Env:    env,
		}
		if tc.preTest != nil {
			tc.preTest(jr)
		}
		if err := jr.Run(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if tc.postTest != nil {
			tc.postTest(jr)
		}

		gotOutput := env.output.String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

var testExecBuilder global.ExecBuilderFunc

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", "./tmp/jsh", "./cmd/jsh")
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to build jsh binary for tests:", err)
		os.Exit(2)
	}
	testExecBuilder = func(source string, args []string) (*exec.Cmd, error) {
		bin := "./tmp/jsh"
		if source != "" {
			args = append([]string{
				"-d", "./test/",
				"-c", source,
				"--"}, args...)
		} else {
			args = append([]string{
				"-d", "./test/",
				args[0],
				"--"}, args[1:]...)
		}
		return exec.Command(bin, args...), nil
	}
	os.Exit(m.Run())
}

func TestJsh(t *testing.T) {
	ts := []TestCase{
		{
			name:   "console_log",
			script: `console.log("Hello, World!");`,
			output: []string{"INFO  Hello, World!"},
		},
		{
			name: "runtime_addShutdownHook",
			script: `
				console.log("Setting shutdown hook");
				runtime.addShutdownHook(function() {
					console.debug("Shutdown hook called");
				});
			`,
			output: []string{
				"INFO  Setting shutdown hook",
				"DEBUG Shutdown hook called",
			},
		},
		{
			name:     "now",
			script:   `const x = console; x.println("NOW:", now());`,
			preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			output: []string{
				"NOW: 2025-12-03 11:22:16",
			},
		},
		{
			name: "module_demo",
			script: `
				const { sayHello } = require("demo");
				sayHello("");
			`,
			output: []string{
				"Hello  from demo.js!",
			},
		},
		{
			name: "node_modules_package_json",
			script: `
				const optparse = require("optparse");
				var SWITCHES = [
					['-h', '--help', 'Show this help message'],
				];
				var parser = new optparse.OptionParser(SWITCHES);
				parser.on('help', function() {
					console.println("Package help");
				});
				parser.parse(['-h']);
			`,
			output: []string{
				"Package help",
			},
		},
		{
			name: "runtime_exec",
			script: `
				runtime.exec("hello.js");
			`,
			output: []string{
				"Hello undefined from demo.js!",
			},
		},
		{
			name: "runtime_exec_args",
			script: `
				runtime.exec("hello.js", "世界");
			`,
			output: []string{
				"Hello 世界 from demo.js!",
			},
		},
		{
			name: "runtime_exec_string",
			script: `
				runtime.execString("console.log('Hello World')");
			`,
			output: []string{
				"INFO  Hello World",
			},
		},
		{
			name: "runtime_exec_string_arg",
			script: `
				runtime.execString("console.log('Hello '+runtime.args[0])", "World");
			`,
			output: []string{
				"INFO  Hello World",
			},
		},
	}

	for _, tc := range ts {
		runTest(t, tc)
	}
}
