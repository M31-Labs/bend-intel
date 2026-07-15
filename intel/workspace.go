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
// `from path import names`; both forms contain an os_path node.
func (d *Document) Imports() []Import {
	var out []Import
	walk(d.Tree.RootNode(), func(n *gotreesitter.Node) {
		typ := n.Type(d.language)
		if typ != "import_name" && typ != "import_from" {
			return
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			child := n.NamedChild(i)
			if child.Type(d.language) == "os_path" {
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
	Root      string
	Documents map[string]*Document
}

func NewWorkspace(root string) *Workspace {
	abs, _ := filepath.Abs(root)
	return &Workspace{Root: abs, Documents: map[string]*Document{}}
}

func (w *Workspace) Add(uri string, source []byte) error {
	doc, err := Parse(source)
	if err != nil {
		return err
	}
	w.Documents[uri] = doc
	return nil
}

func (w *Workspace) Remove(uri string) { delete(w.Documents, uri) }

// Load discovers .bend files below Root. It is cancellation-aware between
// files so editors can abandon a stale workspace scan.
func (w *Workspace) Load(ctx context.Context) error {
	return filepath.WalkDir(w.Root, func(path string, entry os.DirEntry, err error) error {
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
	node := doc.NodeAt(position)
	if node == nil {
		return nil
	}
	for node != nil && node.Type(doc.language) != "identifier" {
		node = node.Parent()
	}
	if node == nil {
		return nil
	}
	name := node.Text(doc.Source)
	var out []WorkspaceLocation
	for candidateURI, candidateDoc := range w.Documents {
		walk(candidateDoc.Tree.RootNode(), func(candidate *gotreesitter.Node) {
			if candidate.Type(candidateDoc.language) == "identifier" && candidate.Text(candidateDoc.Source) == name {
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
	for _, symbol := range w.Symbols() {
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
	node := doc.NodeAt(position)
	if node == nil {
		return nil, fmt.Errorf("no identifier at position")
	}
	for node != nil && node.Type(doc.language) != "identifier" {
		node = node.Parent()
	}
	if node == nil {
		return nil, fmt.Errorf("no identifier at position")
	}
	oldName := node.Text(doc.Source)
	var out []TextEdit
	for candidateURI, candidateDoc := range w.Documents {
		walk(candidateDoc.Tree.RootNode(), func(candidate *gotreesitter.Node) {
			if candidate.Type(candidateDoc.language) == "identifier" && candidate.Text(candidateDoc.Source) == oldName {
				out = append(out, TextEdit{URI: candidateURI, Range: nodeRange(candidate), NewText: newName})
			}
		})
	}
	return out, nil
}

func pathToURI(path string) string { return "file://" + filepath.ToSlash(path) }

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
