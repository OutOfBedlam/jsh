package http

import (
	_ "embed"
	"io"
	"net/http"

	"github.com/dop251/goja"
)

//go:embed http.js
var httpJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	module.Set("NewRequest", NewRequest)
	module.Set("NewAgent", NewAgent)
	module.Set("ReadAll", ReadAll)
	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString("(function(){" + httpJS + "})()")
	if err != nil {
		panic(err)
	}
}

func ReadAll(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	return io.ReadAll(r)
}

func NewRequest(method string, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func NewAgent() *Agent {
	return &Agent{
		client: &http.Client{},
	}
}

type Agent struct {
	client *http.Client
}

func (agent *Agent) Do(req *http.Request) (map[string]any, error) {
	rsp, err := agent.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	rsp.Body.Close()
	headers := map[string]any{}
	for k, v := range rsp.Header {
		if len(v) == 1 {
			headers[k] = v[0]
		} else {
			headers[k] = v
		}
	}
	ret := map[string]any{
		"ok":            rsp.StatusCode >= 200 && rsp.StatusCode < 300,
		"proto":         rsp.Proto,
		"protoMajor":    rsp.ProtoMajor,
		"protoMinor":    rsp.ProtoMinor,
		"statusCode":    rsp.StatusCode,
		"statusMessage": rsp.Status,
		"headers":       headers,
		"body":          &Buffer{data: body},
	}
	return ret, nil
}

type Options struct {
	Hostname string            `json:"hostname"`
	Port     int               `json:"port"`
	Path     string            `json:"path"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
}

type Buffer struct {
	data []byte
}

func (b *Buffer) ToString() string {
	return string(b.data)
}
