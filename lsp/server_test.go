package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestServerInitializeOpenAndSymbols(t *testing.T) {
	input := bytes.NewBuffer(nil)
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend", "version": 1, "text": "def add(x, y):\n  return x + y\n"}}})
	writeFrame(t, input, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///test.bend"}}})
	output := bytes.NewBuffer(nil)
	if err := New(input, output).Run(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{`"name":"bendls"`, `"method":"textDocument/publishDiagnostics"`, `"name":"add"`, `"line":0`} {
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

func writeFrame(t *testing.T, buffer *bytes.Buffer, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(buffer, "Content-Length: %d\r\n\r\n%s", len(body), body)
}
