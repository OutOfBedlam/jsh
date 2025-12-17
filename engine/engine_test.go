package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type TestCase struct {
	name     string
	script   string
	input    []string
	output   []string
	preTest  func(*JSRuntime)
	postTest func(*JSRuntime)
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := Config{
			Name: tc.name,
			Code: tc.script,
			Dir:  "../test/",
			Env: map[string]any{
				"PATH": "/work:/sbin",
				"PWD":  "/work",
			},
			Reader:      &bytes.Buffer{},
			Writer:      &bytes.Buffer{},
			ExecBuilder: testExecBuilder,
		}
		jr, err := New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("process", jr.Process)
		conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, "\n") + "\n")

		if tc.preTest != nil {
			tc.preTest(jr)
		}
		if err := jr.Run(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if tc.postTest != nil {
			tc.postTest(jr)
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
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

var testExecBuilder ExecBuilderFunc

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", "../tmp/jsh", "..")
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to build jsh binary for tests:", err)
		os.Exit(2)
	}
	testExecBuilder = func(source string, args []string) (*exec.Cmd, error) {
		bin := "../tmp/jsh"
		if source != "" {
			args = append([]string{
				"-d", "../test/",
				"-c", source,
				"--"}, args...)
		} else {
			args = append([]string{
				"-d", "../test/",
				args[0],
				"--"}, args[1:]...)
		}
		return exec.Command(bin, args...), nil
	}
	os.Exit(m.Run())
}

func TestJsh(t *testing.T) {
	timeNow, _ := time.ParseInLocation(time.DateTime, "2025-12-03 11:22:16", time.Local)
	ts := []TestCase{
		{
			name:   "console_log",
			script: `console.log("Hello, World!");`,
			output: []string{"INFO  Hello, World!"},
		},
		{
			name: "now",
			script: `
				const x = console;
				const {now} = require("process");
				x.println("NOW:", now());`,
			preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			output: []string{
				fmt.Sprintf("NOW: %s", timeNow.Format(time.DateTime)),
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
			name: "module_package_json",
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
	}

	for _, tc := range ts {
		RunTest(t, tc)
	}
}

func TestSetTimeout(t *testing.T) {
	tests := []TestCase{
		{
			name: "setTimeout_basic",
			script: `
				const {now} = require("process");
				let t = now();
				setTimeout(() => {
					console.log("Timeout executed");
					testDone();
				}, 100);
			`,
			output: []string{
				"INFO  Timeout executed",
			},
		},
		{
			name: "setTimeout_args",
			script: `
				var arg1, arg2;
				setTimeout((a, b) => {
					console.println("Timeout with args:", a, b);
					arg1 = a;
					arg2 = b;
					testDone();
				}, 50,  "test", 42);
			`,
			output: []string{
				"Timeout with args: test 42",
			},
		},
		{
			name: "clearTimeout_basic",
			script: `
				var counter = 0;
				var sum = 0;

				function add(a) {
					counter++;
					sum += a;
					tm = setTimeout(add, 50, a+1);
					if(counter >= 3) {
						clearTimeout(tm);
						setTimeout(()=>{testDone();}, 100);
					}
					console.println("count:", counter,", sum:", sum);					
				}
				var tm = setTimeout(add, 50, 1);
			`,
			output: []string{
				"count: 1 , sum: 1",
				"count: 2 , sum: 3",
				"count: 3 , sum: 6",
			},
		},
		{
			name: "clearTimeout_twice",
			script: `
				var executed = false;
				var tm = setTimeout(()=>{ executed = true; testDone(); }, 50);
				clearTimeout(tm);
				clearTimeout(tm);
				setTimeout(()=>{ testDone(); }, 50); // Ensure test completes
				`,
			output: []string{
				// No output expected regarding execution
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestShutdownHook(t *testing.T) {
	testCases := []TestCase{
		{
			name: "runtime_addShutdownHook",
			script: `
				const {addShutdownHook} = require('process');
				console.log("Setting shutdown hook");
				addShutdownHook(function() {
					console.debug("Shutdown hook called");
				});
			`,
			output: []string{
				"INFO  Setting shutdown hook",
				"DEBUG Shutdown hook called",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}

func TestEventLoop(t *testing.T) {
	testCases := []TestCase{
		{
			name: "eventloop",
			script: `
				console.log("Add event loop");
				setImmediate(() => {
					console.debug("event loop called");
				});
			`,
			output: []string{
				"INFO  Add event loop",
				"DEBUG event loop called",
			},
		},
		{
			// the problem is the nested runOnLoop can not append to the loop
			// while loop is running with mutex lock of the job queue.
			name: "eventloop_loop",
			script: `
				function doIt() {
					console.println("Timeout before doIt");
					setImmediate(() => {
						console.println("event loop called from #1");
						setImmediate(() => {
							console.println("event loop called from #2");
						});
					});
				}
				function doLater() {
					console.println("Event loop after promise resolved");
				}
				console.println("Add event loop");
				setImmediate(() => {
					console.println("Starting doIt");
					setImmediate(() => {
						doIt();
					});
				});
			`,
			output: []string{
				"Add event loop",
				"Starting doIt",
				"Timeout before doIt",
				"event loop called from #1",
				"event loop called from #2",
			},
		},
		{
			name: "eventloop_promise",
			script: `
				const {eventLoop} = require('process');
				function doIt() {
					return new Promise((resolve) => {
						setImmediate(() => {
							console.println("event loop called from promise");
							resolve();
						});
					});
				}
				function doLater() {
					console.println("Event loop after promise resolved");
				}
				console.println("Add event loop");
				doIt().then(() => {
					console.println("Promise resolved");
					setImmediate(doLater);
				});
			`,
			output: []string{
				"Add event loop",
				"event loop called from promise",
				"Promise resolved",
				"Event loop after promise resolved",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}

func TestExec(t *testing.T) {
	testCases := []TestCase{
		{
			name: "runtime_exec",
			script: `
				const {exec} = require('process');
				exec("hello.js");
			`,
			output: []string{
				"Hello undefined from demo.js!",
			},
		},
		{
			name: "runtime_exec_args",
			script: `
				const {exec} = require('process');
				exec("hello.js", "世界");
			`,
			output: []string{
				"Hello 世界 from demo.js!",
			},
		},
		{
			name: "runtime_exec_string",
			script: `
				const {execString} = require('process');
				execString("console.log('Hello World')");
			`,
			output: []string{
				"INFO  Hello World",
			},
		},
		{
			name: "runtime_exec_string_arg",
			script: `
				const {execString} = require('process');
				execString("const {args} = require('process'); console.log('Hello '+args[0])", "World");
			`,
			output: []string{
				"INFO  Hello World",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}
