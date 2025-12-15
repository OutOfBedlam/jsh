package jsh

import (
	"testing"
)

func TestEvents(t *testing.T) {
	tests := []TestCase{
		{
			name: "event_emitter_basic",
			script: `
				const emitter = new EventEmitter();
				emitter.supportedEvents.push("greet");
				
				emitter.on("greet", function(name) {
					console.println("Hello, " + name + "!");
				});

				emitter.emit("greet", "Alice");
				emitter.emit("greet", "Bob");
			`,
			output: []string{
				"Hello, Alice!",
				"Hello, Bob!",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}
