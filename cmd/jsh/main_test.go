package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestCases(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		stdinInput     string
		expectedOutput string
	}{
		{
			name:           "hello with no args",
			args:           []string{"hello"},
			stdinInput:     "",
			expectedOutput: "Hello undefined from demo.js!",
		},
		{
			name:           "hello with no args",
			args:           []string{"hello", "--", "world"},
			stdinInput:     "",
			expectedOutput: "Hello world from demo.js!",
		},
		{
			name:           "exec calls hello with args",
			args:           []string{"exec"},
			stdinInput:     "",
			expectedOutput: "Hello 世界 from demo.js!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare command: go run main.go <args>
			cmdArgs := append([]string{"run", "main.go", "-d", "../../test/"}, tt.args...)
			cmd := exec.Command("go", cmdArgs...)

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
				t.Fatalf("Failed to execute command: %v\nStderr: %s", err, stderr.String())
			}

			// Get the output and trim whitespace
			actualOutput := strings.TrimSpace(stdout.String())
			expectedOutput := strings.TrimSpace(tt.expectedOutput)

			// Compare output with expected
			if actualOutput != expectedOutput {
				t.Errorf("Output mismatch:\nExpected: %q\nActual:   %q", expectedOutput, actualOutput)
			}
		})
	}
}
