package engine

import (
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_env",
			script: `
				const process = require("/lib/process");
				console.println("PATH:", process.env.get("PATH"));
				console.println("PWD:", process.env.get("PWD"));
			`,
			output: []string{
				"PATH: /lib:/work:/sbin",
				"PWD: /work",
			},
		},
		{
			name: "process_argv",
			script: `
				const process = require("/lib/process");
				console.println("argc:", process.argv.length);
				console.println("argv[1]:", process.argv[1]);
			`,
			output: []string{
				"argc: 2",
				"argv[1]: process_argv",
			},
		},
		{
			name: "process_cwd",
			script: `
				const process = require("/lib/process");
				console.println("cwd:", process.cwd());
			`,
			output: []string{
				"cwd: /work",
			},
		},
		{
			name: "process_chdir",
			script: `
				const process = require("/lib/process");
				console.println("before:", process.cwd());
				process.chdir("/lib");
				console.println("after:", process.cwd());
			`,
			output: []string{
				"before: /work",
				"after: /lib",
			},
		},
		{
			name: "process_chdir_relative",
			script: `
				const process = require("/lib/process");
				console.println("before:", process.cwd());
				process.chdir("../lib");
				console.println("after:", process.cwd());
			`,
			output: []string{
				"before: /work",
				"after: /lib",
			},
		},

		{
			name: "process_now",
			script: `
				const process = require("/lib/process");
				const now = process.now();
				console.println("type:", typeof now);
			`,
			preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			output: []string{
				"type: object",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessStdin(t *testing.T) {
	tests := []TestCase{
		{
			name: "stdin_readLines",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				console.println("lines:", lines.length);
				lines.forEach((line, i) => {
					console.println("line", i + ":", line);
				});
			`,
			input: []string{"first line", "second line", "third line"},
			output: []string{
				"lines: 3",
				"line 0: first line",
				"line 1: second line",
				"line 2: third line",
			},
		},
		{
			name: "stdin_readLine",
			script: `
				const process = require("/lib/process");
				const line = process.stdin.readLine();
				console.println("got:", line);
			`,
			input: []string{"hello world"},
			output: []string{
				"got: hello world",
			},
		},
		{
			name: "stdin_read",
			script: `
				const process = require("/lib/process");
				const data = process.stdin.read();
				console.println("length:", data.length);
				const lines = data.split("\n").filter(l => l.length > 0);
				console.println("lines:", lines.length);
			`,
			input: []string{"line1", "line2"},
			output: []string{
				"length: 12",
				"lines: 2",
			},
		},
		{
			name: "stdin_readBytes",
			script: `
				const process = require("/lib/process");
				const data = process.stdin.readBytes(5);
				console.println("read:", data);
				console.println("length:", data.length);
			`,
			input: []string{"hello world"},
			output: []string{
				"read: hello",
				"length: 5",
			},
		},
		{
			name: "stdin_isTTY",
			script: `
				const process = require("/lib/process");
				const isTTY = process.stdin.isTTY();
				console.println("isTTY:", isTTY);
			`,
			input: []string{},
			output: []string{
				"isTTY: false",
			},
		},
		{
			name: "stdin_empty",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				const nonEmpty = lines.filter(l => l.length > 0);
				console.println("non-empty lines:", nonEmpty.length);
			`,
			input: []string{},
			output: []string{
				"non-empty lines: 0",
			},
		},
		{
			name: "stdin_process_lines",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				let total = 0;
				lines.forEach(line => {
					const num = parseInt(line);
					if (!isNaN(num)) {
						total += num;
					}
				});
				console.println("sum:", total);
			`,
			input: []string{"10", "20", "30"},
			output: []string{
				"sum: 60",
			},
		},
		{
			name: "stdin_filter_lines",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				const filtered = lines.filter(line => line.includes("test"));
				console.println("found:", filtered.length);
				filtered.forEach(line => console.println(line));
			`,
			input: []string{"test1", "something", "test2", "other", "testing"},
			output: []string{
				"found: 3",
				"test1",
				"test2",
				"testing",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessExec(t *testing.T) {
	tests := []TestCase{
		{
			name: "exec_basic",
			script: `
				const process = require("/lib/process");
				const exitCode = process.exec("echo", "hello from exec");
				console.println("exit code:", exitCode);
			`,
			output: []string{
				"hello from exec",
				"exit code: 0",
			},
		},
		{
			name: "execString_basic",
			script: `
				const process = require("/lib/process");
				const exitCode = process.execString("console.println('hello from execString')");
				console.println("exit code:", exitCode);
			`,
			output: []string{
				"hello from execString",
				"exit code: 0",
			},
		},
		{
			name: "exec_with_args",
			script: `
				const process = require("/lib/process");
				const exitCode = process.exec("echo", "arg1", "arg2", "arg3");
				console.println("done");
			`,
			output: []string{
				"arg1 arg2 arg3",
				"done",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessShutdownHook(t *testing.T) {
	tests := []TestCase{
		{
			name: "shutdown_hook_single",
			script: `
				const process = require("/lib/process");
				process.addShutdownHook(() => {
					console.println("cleanup");
				});
				console.println("main");
			`,
			output: []string{
				"main",
				"cleanup",
			},
		},
		{
			name: "shutdown_hook_multiple",
			script: `
				const process = require("/lib/process");
				process.addShutdownHook(() => {
					console.println("first hook");
				});
				process.addShutdownHook(() => {
					console.println("second hook");
				});
				console.println("main");
			`,
			output: []string{
				"main",
				"second hook",
				"first hook",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
