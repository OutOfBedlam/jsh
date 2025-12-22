package engine

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestJshMain(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		stdinInput     string
		expectedOutput []string
	}{
		{
			name:           "hello_with_no_args",
			args:           []string{"hello"},
			stdinInput:     "",
			expectedOutput: []string{"Hello  from demo.js!"},
		},
		{
			name:           "hello_with_args",
			args:           []string{"hello", "world"},
			stdinInput:     "",
			expectedOutput: []string{"Hello world from demo.js!"},
		},
		{
			name:           "sbin_echo",
			args:           []string{"echo", "Hello, Echo?"},
			stdinInput:     "",
			expectedOutput: []string{"Hello, Echo?"},
		},
		{
			name:           "exec",
			args:           []string{"exec"},
			stdinInput:     "",
			expectedOutput: []string{"Hello 世界 from demo.js!"},
		},
		{
			name:       "optparse",
			args:       []string{"optparse", "-v", "-h"},
			stdinInput: "",
			expectedOutput: []string{
				"command version 0.1.0",
				"Usage: command [options]",
				"",
				"Available options:",
				"  -h, --help      Show this help message",
				"  -v, --version   Show version information",
				"Options: {help:true, version:true}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare command: go run main.go <args>
			cmdArgs := append([]string{"-v", "/work=../test/"}, tt.args...)
			cmd := exec.Command("../tmp/jsh", cmdArgs...)

			// Setup stdin with bytes.Buffer
			var stdin bytes.Buffer
			stdin.WriteString(tt.stdinInput)
			cmd.Stdin = &stdin

			// Setup stdout with bytes.Buffer
			var stdout bytes.Buffer
			cmd.Stdout = &stdout

			// Setup stderr to capture any errors
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			// Execute the command
			err := cmd.Run()
			if err != nil {
				t.Fatalf("Failed to execute command: %v\n%s", err, stdout.String())
			}

			// Get the output and trim whitespace
			actualOutput := strings.TrimSpace(stdout.String())
			expectedOutput := strings.TrimSpace(strings.Join(tt.expectedOutput, "\n"))

			// Compare output with expected
			if actualOutput != expectedOutput {
				t.Errorf("Output mismatch:\nExpected: %q\nActual:   %q", expectedOutput, actualOutput)
			}
		})
	}
}
