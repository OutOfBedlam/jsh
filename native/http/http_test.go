package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OutOfBedlam/jsh/engine"
)

func echoServer(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/notfound" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case "GET":
		xTestHeader := r.Header.Get("X-Test-Header")            // just to show we can read headers
		w.Header().Set("Date", "Fri, 12 Dec 2025 12:20:01 GMT") // fixed date for testing
		w.Header().Set("X-Test-Header", xTestHeader)
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("echo") != "" {
			w.Write([]byte(r.URL.Query().Get("echo")))
			return
		}
		w.Write([]byte("Hello, World!"))
	case "POST":
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Content-Type must be application/json"))
			return
		}
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		obj := struct {
			Message string `json:"message"`
			Reply   string `json:"reply,omitempty"`
		}{}
		if err := json.Unmarshal(body, &obj); err != nil {
			fmt.Println("echoServer: invalid JSON:", err, ":", string(body))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid JSON"))
			return
		}
		w.WriteHeader(http.StatusOK)
		obj.Reply = "Received"
		b, _ := json.Marshal(&obj) // just to verify it's valid JSON
		w.Write(b)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			Dir:    "../../test/",
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("process", jr.Process)
		jr.RegisterNativeModule("@jsh/http", Module)

		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
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

func TestHttpRequest(t *testing.T) {
	tests := []TestCase{
		{
			name: "http_request_get",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hello?");
				const req = http.request(url);
				req.end((response) => {
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_request_method_url",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hello?");
				const req = http.request(url, {
					host: url.host,
					port: url.port,
					path: url.pathname + url.search,
					method: "get",
					agent: new http.Agent(),
				});
				req.end();
				req.on("response", (response) => {
					if (!response.ok) {
						throw new Error("Request failed with status "+response.statusCode);
					}
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_request_post",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const req = http.request(
					env.get("testURL"),
					{ method:"POST", headers: {"Content-Type":"application/json"} },
				);
				req.on("response", (response) => {
					if (!response.ok) {
						throw new Error("Request failed with status "+response.statusCode);
					}
					const {statusCode, statusMessage} = response;
					console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
					const body = response.json()
					console.println("message:"+ body.message + ", " + "reply:" + body.reply);
				});
				req.on("error", (err) => {
					console.println("Request error:", err.message);
				});
				req.write('{"message": "Hello, ');
				req.end('World!"}');
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"message:Hello, World!, reply:Received",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpGet(t *testing.T) {
	tests := []TestCase{
		{
			name: "http_get_string",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_string_on",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				const req = http.get(url)
				req.on("response", (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_url",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hi?");
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_string_options",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				const opt = {
					headers: {"X-Test-Header": "TestValue"}
				};
				http.get(url, opt, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
					console.println("X-Test-Header:", response.headers["X-Test-Header"]);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"X-Test-Header: TestValue",
			},
		},
		{
			name: "http_get_url_options",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hi?");
				const opt = {
					headers: {"X-Test-Header": "TestValue"}
				};
				http.get(url, opt, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
					console.println("X-Test-Header:", response.headers["X-Test-Header"]);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"X-Test-Header: TestValue",
			},
		},
		{
			name: "http_get_options",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const opt = {
					url: new URL(env.get("testURL")+"?echo=Hi?"),
					headers: {"X-Test-Header": "TestValue"},
				};
				http.get(opt, (response) => {
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
					console.println("Body:", response.string());
					
					contentLength = response.headers["Content-Length"];
					contentType = response.headers["Content-Type"];
					dateHeader = response.headers["Date"];
					if (contentLength != "3") {
						throw new Error("Unexpected Content-Length: "+contentLength);
					}
					if (!/^text\/plain/.test(contentType)) {
						throw new Error("Unexpected Content-Type:"+contentType);
					}
					if (contentType != "text/plain; charset=utf-8") {
						throw new Error("Unexpected Content-Type: "+contentType);
					}
					if (dateHeader != "Fri, 12 Dec 2025 12:20:01 GMT") {
						throw new Error("Unexpected Date header: "+dateHeader);
					}
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"Body: Hi?",
			},
		},
		{
			name: "http_get_not_found",
			script: `
				const http = require("/lib/http");
				const {env} = require('process');
				const url = env.get("testURL")+"/notfound";
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 404",
				"Status: 404 Not Found",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}
