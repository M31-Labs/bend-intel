package lsp

import (
	"bufio"
	"context"
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
	in             *bufio.Reader
	out            io.Writer
	mu             sync.Mutex
	documents      map[string]*intel.Document
	versions       map[string]int32
	shutdown       bool
	workspace      *intel.Workspace
	backend        intel.SemanticBackend
	semanticMu     sync.Mutex
	semanticCancel map[string]context.CancelFunc
	semanticResult map[string]*intel.SemanticResult
}

func New(in io.Reader, out io.Writer) *Server {
	return &Server{in: bufio.NewReader(in), out: out, documents: map[string]*intel.Document{}, versions: map[string]int32{}, workspace: intel.NewWorkspace(""), semanticCancel: map[string]context.CancelFunc{}, semanticResult: map[string]*intel.SemanticResult{}}
}

// SetSemanticBackend installs an optional compiler-backed semantic provider.
// A nil backend keeps the syntax-first server compiler-independent.
func (s *Server) SetSemanticBackend(backend intel.SemanticBackend) { s.backend = backend }

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
		var initParams struct {
			RootURI string `json:"rootUri"`
		}
		_ = json.Unmarshal(msg.Params, &initParams)
		if initParams.RootURI != "" {
			s.workspace = intel.NewWorkspace(intel.PathFromURI(initParams.RootURI))
			_ = s.workspace.Load(context.Background())
		}
		capabilities := map[string]any{"positionEncoding": "utf-16", "textDocumentSync": map[string]any{"openClose": true, "change": 2}, "documentSymbolProvider": true, "foldingRangeProvider": true, "selectionRangeProvider": true, "hoverProvider": true, "definitionProvider": true, "referencesProvider": true, "renameProvider": true, "callHierarchyProvider": true, "signatureHelpProvider": map[string]any{"triggerCharacters": []string{"(", ",", ":"}}, "completionProvider": map[string]any{"triggerCharacters": []string{"/", ".", " "}}, "semanticTokensProvider": map[string]any{"legend": map[string]any{"tokenTypes": intel.SemanticTokenTypes, "tokenModifiers": []string{}}, "full": true}}
		return s.reply(msg.ID, map[string]any{"capabilities": capabilities, "serverInfo": map[string]string{"name": "bendls", "version": "0.1.0"}}, nil)
	case "initialized":
		return nil
	case "shutdown":
		s.cancelSemanticWork()
		s.shutdown = true
		return s.reply(msg.ID, nil, nil)
	case "exit":
		s.cancelSemanticWork()
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
		return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
			text := ""
			s.semanticMu.Lock()
			if result := s.semanticResult[uri]; result != nil {
				text = d.SemanticHover(p, result)
			}
			s.semanticMu.Unlock()
			if text == "" {
				text = d.Hover(p)
			}
			if text == "" {
				return nil
			}
			return map[string]any{"contents": map[string]string{"kind": "markdown", "value": text}}
		})
	case "textDocument/definition":
		return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
			def := s.workspace.Definition(uri, p)
			if def == nil {
				return nil
			}
			return map[string]any{"uri": def.URI, "range": def.Range}
		})
	case "textDocument/references":
		return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
			refs := s.workspace.References(uri, p)
			out := make([]any, 0, len(refs))
			for _, r := range refs {
				out = append(out, map[string]any{"uri": r.URI, "range": r.Range})
			}
			return out
		})
	case "workspace/symbol":
		var params struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		return s.reply(msg.ID, s.workspaceSymbols(params.Query), nil)
	case "textDocument/completion":
		return s.positionRequest(msg, func(uri string, _ *intel.Document, position intel.Position) any {
			return map[string]any{"isIncomplete": false, "items": s.workspace.Completions(uri, position)}
		})
	case "textDocument/rename":
		return s.rename(msg)
	case "textDocument/semanticTokens/full":
		return s.withDocument(msg, func(_ string, d *intel.Document) any {
			data, err := d.SemanticTokens()
			if err != nil {
				return map[string]any{"data": []uint32{}}
			}
			return map[string]any{"data": data}
		})
	case "textDocument/selectionRange":
		return s.selectionRanges(msg)
	case "textDocument/signatureHelp":
		return s.signatureHelp(msg)
	case "callHierarchy/prepare":
		return s.callHierarchyPrepare(msg)
	case "callHierarchy/incomingCalls":
		return s.callHierarchyCalls(msg, true)
	case "callHierarchy/outgoingCalls":
		return s.callHierarchyCalls(msg, false)
	case "bend/parallelStructure":
		return s.withDocument(msg, func(_ string, d *intel.Document) any { return d.ParallelStructure() })
	case "bend/bindingInfo":
		return s.positionRequest(msg, func(_ string, d *intel.Document, p intel.Position) any { return d.BindingInfo(p) })
	case "bend/patternCoverage":
		return s.withDocument(msg, func(_ string, d *intel.Document) any { return d.PatternCoverage() })
	case "bend/hvmView":
		return s.hvmView(msg)
	default:
		if len(msg.ID) > 0 {
			return s.reply(msg.ID, nil, &rpcError{-32601, "method not found: " + msg.Method})
		}
		return nil
	}
}

func (s *Server) hvmView(msg message) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	d := s.documents[params.TextDocument.URI]
	if d == nil {
		return s.reply(msg.ID, nil, nil)
	}
	if backend, ok := s.backend.(intel.SemanticLoweringBackend); ok {
		snapshot := s.semanticSnapshot()
		result, err := backend.Lower(context.Background(), snapshot, params.TextDocument.URI)
		if err == nil && result != nil && result.HVM != "" {
			return s.reply(msg.ID, map[string]any{"kind": "compiler-hvm", "text": result.HVM}, nil)
		}
	}
	return s.reply(msg.ID, map[string]any{"kind": "structural-cst", "text": d.HVMView()}, nil)
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
	_ = s.workspace.Add(p.TextDocument.URI, []byte(p.TextDocument.Text))
	s.semanticMu.Lock()
	s.versions[p.TextDocument.URI] = p.TextDocument.Version
	delete(s.semanticResult, p.TextDocument.URI)
	s.semanticMu.Unlock()
	if err := s.publish(p.TextDocument.URI, d); err != nil {
		return err
	}
	s.scheduleSemantic(p.TextDocument.URI)
	return nil
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
	s.workspace.SetDocument(p.TextDocument.URI, d)
	s.semanticMu.Lock()
	s.versions[p.TextDocument.URI] = p.TextDocument.Version
	delete(s.semanticResult, p.TextDocument.URI)
	s.semanticMu.Unlock()
	if err := s.publish(p.TextDocument.URI, d); err != nil {
		return err
	}
	s.scheduleSemantic(p.TextDocument.URI)
	return nil
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
	s.workspace.Remove(p.TextDocument.URI)
	s.semanticMu.Lock()
	delete(s.versions, p.TextDocument.URI)
	delete(s.semanticResult, p.TextDocument.URI)
	if cancel := s.semanticCancel[p.TextDocument.URI]; cancel != nil {
		cancel()
	}
	delete(s.semanticCancel, p.TextDocument.URI)
	s.semanticMu.Unlock()
	return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{}})
}

func (s *Server) cancelSemanticWork() {
	s.semanticMu.Lock()
	defer s.semanticMu.Unlock()
	for uri, cancel := range s.semanticCancel {
		cancel()
		delete(s.semanticCancel, uri)
	}
}

func (s *Server) publish(uri string, d *intel.Document) error {
	return s.publishDiagnostics(uri, s.versions[uri], d.Diagnostics())
}

func (s *Server) publishDiagnostics(uri string, version int32, diagnostics []intel.Diagnostic) error {
	if diagnostics == nil {
		diagnostics = []intel.Diagnostic{}
	}
	return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "version": version, "diagnostics": diagnostics})
}

func (s *Server) scheduleSemantic(uri string) {
	if s.backend == nil {
		return
	}
	s.semanticMu.Lock()
	if cancel := s.semanticCancel[uri]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.semanticCancel[uri] = cancel
	version := s.versions[uri]
	snapshot := s.semanticSnapshot()
	backend := s.backend
	s.semanticMu.Unlock()
	go func() {
		result, err := backend.Check(ctx, snapshot, uri)
		if err != nil || result == nil || ctx.Err() != nil {
			return
		}
		s.semanticMu.Lock()
		currentVersion, open := s.versions[uri]
		s.semanticMu.Unlock()
		if !open || currentVersion != version {
			return
		}
		for i := range result.Diagnostics {
			if result.Diagnostics[i].Source == "" {
				result.Diagnostics[i].Source = "bend-compiler"
			}
		}
		s.semanticMu.Lock()
		s.semanticResult[uri] = result
		s.semanticMu.Unlock()
		_ = s.publishDiagnostics(uri, version, result.Diagnostics)
	}()
}

func (s *Server) semanticSnapshot() intel.WorkspaceSnapshot {
	snapshot := intel.WorkspaceSnapshot{Root: s.workspace.Root, Documents: map[string]intel.DocumentSnapshot{}}
	for candidateURI, doc := range s.documents {
		snapshot.Documents[candidateURI] = intel.DocumentSnapshot{URI: candidateURI, Version: s.versions[candidateURI], Source: append([]byte(nil), doc.Source...)}
	}
	return snapshot
}

func (s *Server) signatureHelp(msg message) error {
	return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
		s.semanticMu.Lock()
		result := s.semanticResult[uri]
		s.semanticMu.Unlock()
		if result == nil {
			return map[string]any{"signatures": []any{}, "activeSignature": 0, "activeParameter": 0}
		}
		name := d.IdentifierNameAt(p)
		for _, signature := range result.Signatures {
			if signature.Name != name {
				continue
			}
			label := signature.Name
			if len(signature.Parameters) > 0 {
				label += "(" + strings.Join(signature.Parameters, ", ") + ")"
			}
			if signature.ReturnType != "" {
				label += " -> " + signature.ReturnType
			}
			return map[string]any{"signatures": []any{map[string]any{"label": label}}, "activeSignature": 0, "activeParameter": 0}
		}
		return map[string]any{"signatures": []any{}, "activeSignature": 0, "activeParameter": 0}
	})
}

func (s *Server) callHierarchyPrepare(msg message) error {
	return s.positionRequest(msg, func(uri string, d *intel.Document, p intel.Position) any {
		name := d.IdentifierNameAt(p)
		if name == "" {
			return []any{}
		}
		for _, symbol := range d.Symbols() {
			if symbol.Name == name {
				return []any{map[string]any{"name": symbol.Name, "kind": symbolKind(symbol.Kind), "uri": uri, "range": symbol.Range, "selectionRange": symbol.Range, "data": map[string]any{"uri": uri, "name": name}}}
			}
		}
		return []any{}
	})
}

func (s *Server) callHierarchyCalls(msg message, incoming bool) error {
	var params struct {
		Item struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"item"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	var position intel.Position
	if doc := s.documents[params.Item.URI]; doc != nil {
		for _, symbol := range doc.Symbols() {
			if symbol.Name == params.Item.Name {
				position = symbol.Range.Start
				break
			}
		}
	}
	incomingCalls, outgoingCalls := s.workspace.CallHierarchy(params.Item.URI, position)
	if incoming {
		out := make([]any, 0, len(incomingCalls))
		for _, call := range incomingCalls {
			from := s.callHierarchyItem(call.URI, call.Caller, call.Range)
			out = append(out, map[string]any{"from": from, "fromRanges": []intel.Range{call.Range}})
		}
		return s.reply(msg.ID, out, nil)
	}
	from := s.callHierarchyItem(params.Item.URI, params.Item.Name, positionRange(position))
	out := make([]any, 0, len(outgoingCalls))
	for _, call := range outgoingCalls {
		to := s.callHierarchyItem(call.URI, call.Callee, call.Range)
		out = append(out, map[string]any{"from": from, "to": to, "fromRanges": []intel.Range{call.Range}})
	}
	return s.reply(msg.ID, out, nil)
}

func (s *Server) callHierarchyItem(uri, name string, fallback intel.Range) map[string]any {
	if doc := s.documents[uri]; doc != nil {
		for _, symbol := range doc.Symbols() {
			if symbol.Name == name {
				return map[string]any{"name": symbol.Name, "kind": symbolKind(symbol.Kind), "uri": uri, "range": symbol.Range, "selectionRange": symbol.Range, "data": map[string]any{"uri": uri, "name": name}}
			}
		}
	}
	kind := 12
	if name == "<module>" {
		kind = 2
	}
	return map[string]any{"name": name, "kind": kind, "uri": uri, "range": fallback, "selectionRange": fallback, "data": map[string]any{"uri": uri, "name": name}}
}

func positionRange(position intel.Position) intel.Range {
	return intel.Range{Start: position, End: position}
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

func (s *Server) workspaceSymbols(query string) []any {
	out := make([]any, 0)
	for _, symbol := range s.workspace.Symbols() {
		if query != "" && !strings.Contains(strings.ToLower(symbol.Name), strings.ToLower(query)) {
			continue
		}
		out = append(out, map[string]any{"name": symbol.Name, "kind": symbolKind(symbol.Kind), "location": map[string]any{"uri": symbol.URI, "range": symbol.Range}})
	}
	return out
}

func (s *Server) selectionRanges(msg message) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Positions []intel.Position `json:"positions"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	doc := s.documents[params.TextDocument.URI]
	if doc == nil {
		return s.reply(msg.ID, []any{}, nil)
	}
	return s.reply(msg.ID, doc.SelectionRanges(params.Positions), nil)
}

func symbolKind(kind string) int {
	switch kind {
	case "function":
		return 12
	case "type":
		return 23
	case "object":
		return 5
	default:
		return 13
	}
}

func (s *Server) rename(msg message) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position intel.Position `json:"position"`
		NewName  string         `json:"newName"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	edits, err := s.workspace.Rename(params.TextDocument.URI, params.Position, params.NewName)
	if err != nil {
		return s.reply(msg.ID, nil, &rpcError{-32602, err.Error()})
	}
	changes := map[string][]map[string]any{}
	for _, edit := range edits {
		changes[edit.URI] = append(changes[edit.URI], map[string]any{"range": edit.Range, "newText": edit.NewText})
	}
	return s.reply(msg.ID, map[string]any{"changes": changes}, nil)
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
	server := New(os.Stdin, os.Stdout)
	if command := strings.TrimSpace(os.Getenv("BEND_SEMANTICD")); command != "" {
		server.SetSemanticBackend(intel.NewSemanticCommandBackend(command))
	}
	if err := server.Run(); err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "bendls:", err)
		os.Exit(1)
	}
}
