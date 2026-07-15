package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/M31-Labs/bend-intel/intel"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	in        *bufio.Reader
	out       io.Writer
	mu        sync.Mutex
	documents map[string]*intel.Document
	versions  map[string]int32
	shutdown  bool
}

func New(in io.Reader, out io.Writer) *Server {
	return &Server{in: bufio.NewReader(in), out: out, documents: map[string]*intel.Document{}, versions: map[string]int32{}}
}

func (s *Server) Run() error {
	for {
		msg, err := s.read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.handle(msg); err != nil {
			if err == io.EOF {
				return nil
			}
			if len(msg.ID) > 0 {
				_ = s.reply(msg.ID, nil, &rpcError{-32603, err.Error()})
			}
		}
	}
}

func (s *Server) handle(msg message) error {
	switch msg.Method {
	case "initialize":
		capabilities := map[string]any{"positionEncoding": "utf-16", "textDocumentSync": map[string]any{"openClose": true, "change": 2}, "documentSymbolProvider": true, "foldingRangeProvider": true, "hoverProvider": true, "definitionProvider": true, "referencesProvider": true, "semanticTokensProvider": map[string]any{"legend": map[string]any{"tokenTypes": intel.SemanticTokenTypes, "tokenModifiers": []string{}}, "full": true}}
		return s.reply(msg.ID, map[string]any{"capabilities": capabilities, "serverInfo": map[string]string{"name": "bendls", "version": "0.1.0"}}, nil)
	case "initialized":
		return nil
	case "shutdown":
		s.shutdown = true
		return s.reply(msg.ID, nil, nil)
	case "exit":
		return io.EOF
	case "textDocument/didOpen":
		return s.didOpen(msg.Params)
	case "textDocument/didChange":
		return s.didChange(msg.Params)
	case "textDocument/didClose":
		return s.didClose(msg.Params)
	case "textDocument/documentSymbol":
		return s.documentSymbols(msg)
	case "textDocument/foldingRange":
		return s.withDocument(msg, func(_ string, d *intel.Document) any { return d.FoldingRanges() })
	case "textDocument/hover":
		return s.positionRequest(msg, func(_ string, d *intel.Document, p intel.Position) any {
			text := d.Hover(p)
			if text == "" {
				return nil
			}
			return map[string]any{"contents": map[string]string{"kind": "markdown", "value": text}}
		})
	case "textDocument/definition":
		return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
			def := d.Definition(p)
			if def == nil {
				return nil
			}
			return map[string]any{"uri": uri, "range": def.Range}
		})
	case "textDocument/references":
		return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
			refs := d.References(p)
			out := make([]any, 0, len(refs))
			for _, r := range refs {
				out = append(out, map[string]any{"uri": uri, "range": r})
			}
			return out
		})
	case "textDocument/semanticTokens/full":
		return s.withDocument(msg, func(_ string, d *intel.Document) any {
			data, err := d.SemanticTokens()
			if err != nil {
				return map[string]any{"data": []uint32{}}
			}
			return map[string]any{"data": data}
		})
	default:
		if len(msg.ID) > 0 {
			return s.reply(msg.ID, nil, &rpcError{-32601, "method not found: " + msg.Method})
		}
		return nil
	}
}

func (s *Server) didOpen(raw json.RawMessage) error {
	var p struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Text    string `json:"text"`
			Version int32  `json:"version"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	d, err := intel.Parse([]byte(p.TextDocument.Text))
	if err != nil {
		return err
	}
	s.documents[p.TextDocument.URI] = d
	s.versions[p.TextDocument.URI] = p.TextDocument.Version
	return s.publish(p.TextDocument.URI, d)
}

func (s *Server) didChange(raw json.RawMessage) error {
	var p struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int32  `json:"version"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Range *intel.Range `json:"range"`
			Text  string       `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return fmt.Errorf("document not open: %s", p.TextDocument.URI)
	}
	changes := make([]intel.TextChange, len(p.ContentChanges))
	for i, c := range p.ContentChanges {
		changes[i] = intel.TextChange{Range: c.Range, Text: c.Text}
	}
	if err := d.ApplyChanges(changes); err != nil {
		return err
	}
	s.versions[p.TextDocument.URI] = p.TextDocument.Version
	return s.publish(p.TextDocument.URI, d)
}

func (s *Server) didClose(raw json.RawMessage) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	delete(s.documents, p.TextDocument.URI)
	delete(s.versions, p.TextDocument.URI)
	return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{}})
}
func (s *Server) publish(uri string, d *intel.Document) error {
	return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "version": s.versions[uri], "diagnostics": d.Diagnostics()})
}

func (s *Server) documentSymbols(msg message) error {
	return s.withDocument(msg, func(_ string, d *intel.Document) any {
		symbols := d.Symbols()
		out := make([]any, 0, len(symbols))
		kinds := map[string]int{"function": 12, "type": 23, "object": 5}
		for _, symbol := range symbols {
			out = append(out, map[string]any{"name": symbol.Name, "kind": kinds[symbol.Kind], "range": symbol.Range, "selectionRange": symbol.Range})
		}
		return out
	})
}
func (s *Server) withDocument(msg message, fn func(string, *intel.Document) any) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return s.reply(msg.ID, nil, nil)
	}
	return s.reply(msg.ID, fn(p.TextDocument.URI, d), nil)
}
func (s *Server) positionRequest(msg message, fn func(string, *intel.Document, intel.Position) any) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position intel.Position `json:"position"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return s.reply(msg.ID, nil, nil)
	}
	return s.reply(msg.ID, fn(p.TextDocument.URI, d, p.Position), nil)
}

func (s *Server) read() (message, error) {
	length := -1
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return message{}, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Content-Length") {
			length, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
	}
	if length < 0 {
		return message{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.in, body); err != nil {
		return message{}, err
	}
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		return message{}, err
	}
	return msg, nil
}
func (s *Server) reply(id json.RawMessage, result any, rpcErr *rpcError) error {
	message := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		message["error"] = rpcErr
	} else {
		message["result"] = result
	}
	return s.write(message)
}
func (s *Server) notify(method string, params any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (s *Server) write(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func Main() {
	if err := New(os.Stdin, os.Stdout).Run(); err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "bendls:", err)
		os.Exit(1)
	}
}
