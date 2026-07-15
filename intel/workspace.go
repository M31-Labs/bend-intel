package intel

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// Import is a syntax-level Bend import. The resolver intentionally keeps the
// written path so it can report unresolved imports without pretending to be
// the Bend compiler's package loader.
type Import struct {
	Path  string `json:"path"`
	Range Range  `json:"range"`
}

// Imports extracts import paths from the CST. Bend has both `import path` and
// `from path import names`; current Bend grammar versions represent the path
// as a full identifier with a slash/path child.
func (d *Document) Imports() []Import {
	var out []Import
	walk(d.Tree.RootNode(), func(n *gotreesitter.Node) {
		typ := n.Type(d.language)
		if typ != "import_name" && typ != "import_from" {
			return
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			child := n.NamedChild(i)
			if child.Type(d.language) == "identifier" {
				out = append(out, Import{Path: child.Text(d.Source), Range: nodeRange(child)})
				break
			}
		}
	})
	return out
}

// Workspace is the editor-independent project index. It only owns syntax
// facts; Bend's compiler remains responsible for package/type semantics.
type Workspace struct {
	Root        string
	Documents   map[string]*Document
	ImportGraph ImportGraph
}

// ImportGraph records only paths resolved to indexed documents. The syntax
// layer still exposes unresolved imports through Document.Imports.
type ImportGraph struct {
	Edges   map[string][]string
	Reverse map[string][]string
}

func NewWorkspace(root string) *Workspace {
	abs, _ := filepath.Abs(root)
	return &Workspace{Root: abs, Documents: map[string]*Document{}, ImportGraph: ImportGraph{Edges: map[string][]string{}, Reverse: map[string][]string{}}}
}

func (w *Workspace) Add(uri string, source []byte) error {
	doc, err := Parse(source)
	if err != nil {
		return err
	}
	w.Documents[uri] = doc
	w.rebuildImportGraph()
	return nil
}

func (w *Workspace) Remove(uri string) {
	delete(w.Documents, uri)
	w.rebuildImportGraph()
}

// SetDocument replaces an open document and refreshes import edges. Editors
// use this after an incremental edit because the import set can itself change.
func (w *Workspace) SetDocument(uri string, doc *Document) {
	if doc == nil {
		w.Remove(uri)
		return
	}
	w.Documents[uri] = doc
	w.rebuildImportGraph()
}

// Load discovers .bend files below Root. It is cancellation-aware between
// files so editors can abandon a stale workspace scan.
func (w *Workspace) Load(ctx context.Context) error {
	err := filepath.WalkDir(w.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if path != w.Root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".bend" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		uri := pathToURI(path)
		return w.Add(uri, source)
	})
	w.rebuildImportGraph()
	return err
}

type WorkspaceSymbol struct {
	URI string `json:"uri"`
	Symbol
}

func (w *Workspace) Symbols() []WorkspaceSymbol {
	var out []WorkspaceSymbol
	for uri, doc := range w.Documents {
		for _, symbol := range doc.Symbols() {
			out = append(out, WorkspaceSymbol{URI: uri, Symbol: symbol})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].URI < out[j].URI
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (w *Workspace) Definition(uri string, position Position) *WorkspaceSymbol {
	doc := w.Documents[uri]
	if doc == nil {
		return nil
	}
	local := doc.Definition(position)
	if node := identifierAt(doc, position); node != nil {
		name := node.Text(doc.Source)
		if binding := doc.Scopes().bindingAt(node, name); binding != nil && binding.ScopeID != 0 {
			if local == nil {
				local = &Symbol{Name: binding.Name, Kind: binding.Kind, Range: binding.Range}
			}
			return &WorkspaceSymbol{URI: uri, Symbol: *local}
		}
		if local != nil && local.Name == name {
			return &WorkspaceSymbol{URI: uri, Symbol: *local}
		}
		for candidateURI := range w.relatedDocuments(uri) {
			if symbol := w.symbol(candidateURI, name); symbol != nil {
				return &WorkspaceSymbol{URI: candidateURI, Symbol: *symbol}
			}
		}
	}
	if local == nil {
		return nil
	}
	for _, symbol := range w.Symbols() {
		if symbol.Name == local.Name {
			copy := symbol
			return &copy
		}
	}
	return nil
}

func (w *Workspace) References(uri string, position Position) []WorkspaceLocation {
	doc := w.Documents[uri]
	if doc == nil {
		return nil
	}
	node := identifierAt(doc, position)
	if node == nil {
		return nil
	}
	name := node.Text(doc.Source)
	target := doc.Scopes().bindingAt(node, name)
	related := w.relatedDocuments(w.targetURI(uri, name, target))
	var out []WorkspaceLocation
	for candidateURI, candidateDoc := range w.Documents {
		if !w.candidateDocumentAllowed(candidateURI, uri, target, related) {
			continue
		}
		walk(candidateDoc.Tree.RootNode(), func(candidate *gotreesitter.Node) {
			if candidate.Type(candidateDoc.language) != "identifier" || candidate.Text(candidateDoc.Source) != name {
				return
			}
			if w.candidateMatches(candidateURI, uri, candidateDoc.Scopes().bindingAt(candidate, name), target, name) {
				out = append(out, WorkspaceLocation{URI: candidateURI, Range: nodeRange(candidate)})
			}
		})
	}
	return out
}

type WorkspaceLocation struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// CallHierarchy returns syntax-derived call sites across indexed documents.
// It intentionally includes only currently indexed files; unopened files are
// loaded by Workspace.Load when a root is configured.
func (w *Workspace) CallHierarchy(uri string, position Position) (incoming []WorkspaceCall, outgoing []WorkspaceCall) {
	doc := w.Documents[uri]
	if doc == nil {
		return nil, nil
	}
	node := identifierAt(doc, position)
	name := ""
	if node != nil {
		name = node.Text(doc.Source)
	} else {
		// LSP call-hierarchy items carry the whole definition range, whose
		// start is often the `def` keyword rather than the name identifier.
		// Accept that stable range as a fallback for clients that round-trip
		// the item without preserving the prepare cursor.
		for _, symbol := range doc.Symbols() {
			if positionLE(symbol.Range.Start, position) && positionLE(position, symbol.Range.End) {
				name = symbol.Name
				break
			}
		}
	}
	if name == "" {
		return nil, nil
	}
	for candidateURI, candidateDoc := range w.Documents {
		for _, call := range candidateDoc.CallSites() {
			if call.Callee == name {
				incoming = append(incoming, WorkspaceCall{URI: candidateURI, Caller: call.Caller, Callee: call.Callee, Range: call.Range})
			}
			if candidateURI == uri && call.Caller == name {
				outgoing = append(outgoing, WorkspaceCall{URI: candidateURI, Caller: call.Caller, Callee: call.Callee, Range: call.Range})
			}
		}
	}
	sort.Slice(incoming, func(i, j int) bool { return locationLess(incoming[i].Range, incoming[j].Range) })
	sort.Slice(outgoing, func(i, j int) bool { return locationLess(outgoing[i].Range, outgoing[j].Range) })
	return incoming, outgoing
}

type WorkspaceCall struct {
	URI    string `json:"uri"`
	Caller string `json:"caller"`
	Callee string `json:"callee"`
	Range  Range  `json:"range"`
}

func locationLess(a, b Range) bool {
	if a.Start.Line == b.Start.Line {
		return a.Start.Character < b.Start.Character
	}
	return a.Start.Line < b.Start.Line
}

type TextEdit struct {
	URI     string `json:"uri"`
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

var bendKeywords = []string{"def", "type", "object", "hvm", "import", "from", "return", "match", "case", "switch", "bend", "fold", "when", "else", "with", "ask", "use", "open", "lambda"}

type Completion struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

func (w *Workspace) Completions(uri string, position Position) []Completion {
	doc := w.Documents[uri]
	if doc == nil {
		return nil
	}
	var out []Completion
	seen := map[string]bool{}
	for _, keyword := range bendKeywords {
		seen[keyword] = true
		out = append(out, Completion{Label: keyword, Kind: 14, Detail: "Bend keyword"})
	}
	for _, binding := range doc.Scopes().Bindings {
		if binding.ScopeID == 0 || seen[binding.Name] {
			continue
		}
		seen[binding.Name] = true
		out = append(out, Completion{Label: binding.Name, Kind: 6, Detail: binding.Kind})
	}
	related := w.relatedDocuments(uri)
	for _, symbol := range w.Symbols() {
		if !related[symbol.URI] {
			continue
		}
		if seen[symbol.Name] {
			continue
		}
		seen[symbol.Name] = true
		kind := 3
		if symbol.Kind == "function" {
			kind = 3
		}
		if symbol.Kind == "type" {
			kind = 7
		}
		out = append(out, Completion{Label: symbol.Name, Kind: kind, Detail: symbol.Kind})
	}
	return out
}

func (w *Workspace) Rename(uri string, position Position, newName string) ([]TextEdit, error) {
	if strings.TrimSpace(newName) == "" {
		return nil, fmt.Errorf("rename target cannot be empty")
	}
	doc := w.Documents[uri]
	if doc == nil {
		return nil, fmt.Errorf("document not found: %s", uri)
	}
	node := identifierAt(doc, position)
	if node == nil {
		return nil, fmt.Errorf("no identifier at position")
	}
	oldName := node.Text(doc.Source)
	target := doc.Scopes().bindingAt(node, oldName)
	related := w.relatedDocuments(w.targetURI(uri, oldName, target))
	var out []TextEdit
	for candidateURI, candidateDoc := range w.Documents {
		if !w.candidateDocumentAllowed(candidateURI, uri, target, related) {
			continue
		}
		walk(candidateDoc.Tree.RootNode(), func(candidate *gotreesitter.Node) {
			if candidate.Type(candidateDoc.language) != "identifier" || candidate.Text(candidateDoc.Source) != oldName {
				return
			}
			if w.candidateMatches(candidateURI, uri, candidateDoc.Scopes().bindingAt(candidate, oldName), target, oldName) {
				out = append(out, TextEdit{URI: candidateURI, Range: nodeRange(candidate), NewText: newName})
			}
		})
	}
	return out, nil
}

func pathToURI(path string) string { return "file://" + filepath.ToSlash(path) }

func identifierAt(doc *Document, position Position) *gotreesitter.Node {
	node := doc.NodeAt(position)
	for node != nil && node.Type(doc.language) != "identifier" {
		node = node.Parent()
	}
	return node
}

func (w *Workspace) targetURI(uri, name string, target *Binding) string {
	if target != nil || w.symbol(uri, name) != nil {
		return uri
	}
	for candidateURI := range w.relatedDocuments(uri) {
		if w.symbol(candidateURI, name) != nil {
			return candidateURI
		}
	}
	return uri
}

func (w *Workspace) candidateDocumentAllowed(candidateURI, requestURI string, target *Binding, related map[string]bool) bool {
	if target != nil && target.ScopeID != 0 {
		return candidateURI == requestURI
	}
	return related[candidateURI]
}

func (w *Workspace) candidateMatches(candidateURI, requestURI string, candidate, target *Binding, name string) bool {
	if target != nil && target.ScopeID != 0 {
		return candidateURI == requestURI && sameBinding(target, candidate)
	}
	return candidate == nil || candidate.ScopeID == 0 && candidate.Name == name
}

func (w *Workspace) symbol(uri, name string) *Symbol {
	doc := w.Documents[uri]
	if doc == nil {
		return nil
	}
	for _, symbol := range doc.Symbols() {
		if symbol.Name == name {
			copy := symbol
			return &copy
		}
	}
	return nil
}

func (w *Workspace) rebuildImportGraph() {
	graph := ImportGraph{Edges: map[string][]string{}, Reverse: map[string][]string{}}
	for uri, doc := range w.Documents {
		for _, imp := range doc.Imports() {
			if target, ok := w.resolveImport(uri, imp.Path); ok {
				graph.Edges[uri] = append(graph.Edges[uri], target)
				graph.Reverse[target] = append(graph.Reverse[target], uri)
			}
		}
	}
	for uri := range graph.Edges {
		sort.Strings(graph.Edges[uri])
	}
	for uri := range graph.Reverse {
		sort.Strings(graph.Reverse[uri])
	}
	w.ImportGraph = graph
}

func (w *Workspace) resolveImport(fromURI, importPath string) (string, bool) {
	path := strings.Trim(strings.TrimSpace(importPath), "\"'")
	if path == "" {
		return "", false
	}
	from := PathFromURI(fromURI)
	var candidates []string
	if filepath.IsAbs(from) {
		candidates = append(candidates, filepath.Join(filepath.Dir(from), filepath.FromSlash(path)))
	}
	if w.Root != "" {
		candidates = append(candidates, filepath.Join(w.Root, filepath.FromSlash(path)))
	}
	for _, candidate := range candidates {
		for _, variant := range []string{candidate, candidate + ".bend", filepath.Join(candidate, "main.bend")} {
			uri := pathToURI(filepath.Clean(variant))
			if _, ok := w.Documents[uri]; ok {
				return uri, true
			}
		}
	}
	return "", false
}

func (w *Workspace) relatedDocuments(uri string) map[string]bool {
	related := map[string]bool{uri: true}
	for _, target := range w.ImportGraph.Edges[uri] {
		related[target] = true
	}
	for _, source := range w.ImportGraph.Reverse[uri] {
		related[source] = true
	}
	return related
}

// PathFromURI converts a file URI received from an editor into an OS path.
func PathFromURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return uri
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil || path == "" {
		return uri
	}
	return filepath.FromSlash(path)
}
