package main

import (
	"context"
	"os/exec"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
)

type lspClient struct {
	conn *jsonrpc2.Conn
	mu   sync.Mutex
}

type lspHandler struct{}

func (h *lspHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if req.Method == "textDocument/publishDiagnostics" {
		// In a real implementation, we would parse the diagnostics and update the UI.
		// For this prototype, we just set a status message.
		setStatus("LSP: Received diagnostics")
	}
}

func startLSP() *lspClient {
	cmd := exec.Command("gopls", "serve")
	stdin, err := cmd.StdinPipe()
	if err != nil { return nil }
	stdout, err := cmd.StdoutPipe()
	if err != nil { return nil }
	if err := cmd.Start(); err != nil { return nil }

	handler := &lspHandler{}
	stream := jsonrpc2.NewBufferedStream(&rwWrapper{stdin, stdout}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(context.Background(), stream, handler)
	
	client := &lspClient{conn: conn}
	client.initialize()
	return client
}

func (c *lspClient) initialize() {
	if c == nil || c.conn == nil { return }
	var result interface{}
	_ = c.conn.Call(context.Background(), "initialize", map[string]interface{}{
		"processId": 0,
		"capabilities": map[string]interface{}{},
	}, &result)
	_ = c.conn.Notify(context.Background(), "initialized", map[string]interface{}{})
}

type rwWrapper struct {
	stdin  interface{ Write([]byte) (int, error) }
	stdout interface{ Read([]byte) (int, error) }
}

func (w *rwWrapper) Read(p []byte) (n int, err error)  { return w.stdout.Read(p) }
func (w *rwWrapper) Write(p []byte) (n int, err error) { return w.stdin.Write(p) }
func (w *rwWrapper) Close() error                      { return nil }
