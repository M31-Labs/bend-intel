package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/M31-Labs/bend-intel/intel"
)

func TestServerInitializeOpenAndSymbols(t *testing.T) {
	input := bytes.NewBuffer(nil)
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend", "version": 1, "text": "def add(x, y):\n  return x + y\n"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "textDocument/semanticTokens/full", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "textDocument/selectionRange", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend"}, "positions": []any{map[string]any{"line": 1, "character": 9}}}})
	output := bytes.NewBuffer(nil)
	if err := New(input, output).Run(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{`"name":"bendls"`, `"method":"textDocument/publishDiagnostics"`, `"name":"add"`, `"line":0`, `"data":[`, `"parent"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %s:\n%s", want, got)
		}
	}
}

func TestServerAppliesIncrementalChange(t *testing.T) {
	input := bytes.NewBuffer(nil)
	writeFrame(t, input, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend", "version": 1, "text": "def old(x):\n  return x\n"}},
	})
	writeFrame(t, input, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didChange",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///test.bend", "version": 2},
			"contentChanges": []any{map[string]any{
				"range": map[string]any{"start": map[string]any{"line": 0, "character": 4}, "end": map[string]any{"line": 0, "character": 7}},
				"text":  "new",
			}},
		},
	})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend"}}})
	output := bytes.NewBuffer(nil)
	if err := New(input, output).Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"name":"new"`) {
		t.Fatalf("updated symbol missing:\n%s", output.String())
	}
}

func TestShutdownReturnsNullResultAndExitStops(t *testing.T) {
	input := bytes.NewBuffer(nil)
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "shutdown"})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	output := bytes.NewBuffer(nil)
	if err := New(input, output).Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"result":null`) {
		t.Fatalf("shutdown response missing null result: %s", output.String())
	}
}

func TestWorkspaceSymbolsCompletionAndRename(t *testing.T) {
	input := bytes.NewBuffer(nil)
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///main.bend", "version": 1, "text": "import defs\ndef main():\n  return add(1, 2)\n"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///defs.bend", "version": 1, "text": "def add(x, y):\n  return x + y\n"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "workspace/symbol", "params": map[string]any{"query": "add"}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "textDocument/completion", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///main.bend"}, "position": map[string]any{"line": 2, "character": 9}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 6, "method": "textDocument/rename", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///main.bend"}, "position": map[string]any{"line": 2, "character": 9}, "newName": "sum"}})
	output := bytes.NewBuffer(nil)
	if err := New(input, output).Run(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"location"`, `"label":"add"`, `"changes"`, `"newText":"sum"`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("workspace output missing %s:\n%s", want, output.String())
		}
	}
}

func writeFrame(t *testing.T, buffer *bytes.Buffer, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(buffer, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

type testSemanticBackend struct {
	started      chan int
	releaseFirst chan struct{}
	calls        atomic.Int32
}

func (b *testSemanticBackend) Check(_ context.Context, _ intel.WorkspaceSnapshot, _ string) (*intel.SemanticResult, error) {
	call := int(b.calls.Add(1))
	b.started <- call
	if call == 1 {
		<-b.releaseFirst
		return &intel.SemanticResult{Diagnostics: []intel.Diagnostic{{Message: "stale", Range: intel.Range{}, Severity: 1}}}, nil
	}
	return &intel.SemanticResult{Diagnostics: []intel.Diagnostic{{Message: "current", Range: intel.Range{}, Severity: 1}}}, nil
}

func TestSemanticBackendDropsStaleVersionResults(t *testing.T) {
	input := bytes.NewBuffer(nil)
	output := bytes.NewBuffer(nil)
	backend := &testSemanticBackend{started: make(chan int, 2), releaseFirst: make(chan struct{})}
	s := New(input, output)
	s.SetSemanticBackend(backend)
	open := map[string]any{"textDocument": map[string]any{"uri": "file:///semantic.bend", "version": 1, "text": "def main():\n  return 1\n"}}
	raw, _ := json.Marshal(open)
	if err := s.didOpen(raw); err != nil {
		t.Fatal(err)
	}
	if got := <-backend.started; got != 1 {
		t.Fatalf("first backend call = %d", got)
	}
	change := map[string]any{"textDocument": map[string]any{"uri": "file:///semantic.bend", "version": 2}, "contentChanges": []any{map[string]any{"text": "def main():\n  return 2\n"}}}
	raw, _ = json.Marshal(change)
	if err := s.didChange(raw); err != nil {
		t.Fatal(err)
	}
	if got := <-backend.started; got != 2 {
		t.Fatalf("second backend call = %d", got)
	}
	close(backend.releaseFirst)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !strings.Contains(safeOutput(s, output), `"message":"current"`) {
		time.Sleep(time.Millisecond)
	}
	got := safeOutput(s, output)
	if !strings.Contains(got, `"message":"current"`) {
		t.Fatalf("current semantic result missing: %s", got)
	}
	if strings.Contains(got, `"message":"stale"`) {
		t.Fatalf("stale semantic result published: %s", got)
	}
}

func safeOutput(s *Server, output *bytes.Buffer) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return output.String()
}
