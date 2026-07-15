package intel

import (
	"fmt"

	"github.com/M31-Labs/bend-intel/bendlang"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

type Position struct{ Line, Character uint32 }
type Range struct{ Start, End Position }
type Diagnostic struct {
	Message string `json:"message"`
	Range   Range  `json:"range"`
}
type Symbol struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Range Range  `json:"range"`
}

type Document struct {
	Source   []byte
	Tree     *gotreesitter.Tree
	language *gotreesitter.Language
}

func Parse(source []byte) (*Document, error) {
	lang, err := bendlang.Language()
	if err != nil {
		return nil, err
	}
	parser, err := bendlang.NewParser()
	if err != nil {
		return nil, err
	}
	tree, err := parser.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse Bend: %w", err)
	}
	return &Document{Source: append([]byte(nil), source...), Tree: tree, language: lang}, nil
}

func (d *Document) Diagnostics() []Diagnostic {
	var out []Diagnostic
	collectInnermostErrors(d.Tree.RootNode(), &out)
	return out
}

// collectInnermostErrors suppresses the nested ERROR wrappers produced during
// recovery. Authors need the smallest actionable range, not every ancestor of
// the same parser failure.
func collectInnermostErrors(n *gotreesitter.Node, out *[]Diagnostic) bool {
	if n == nil {
		return false
	}
	hasErrorChild := false
	for i := 0; i < n.NamedChildCount(); i++ {
		if collectInnermostErrors(n.NamedChild(i), out) {
			hasErrorChild = true
		}
	}
	if n.IsError() && !hasErrorChild {
		*out = append(*out, Diagnostic{Message: "unexpected Bend syntax", Range: nodeRange(n)})
	}
	return n.IsError() || hasErrorChild
}

func (d *Document) Symbols() []Symbol {
	kinds := map[string]string{"imp_function_definition": "function", "fun_function_definition": "function", "hvm_definition": "function", "imp_type_definition": "type", "fun_type_definition": "type", "object_definition": "object"}
	var out []Symbol
	walk(d.Tree.RootNode(), func(n *gotreesitter.Node) {
		kind, ok := kinds[n.Type(d.language)]
		if !ok {
			return
		}
		name := n.ChildByFieldName("name", d.language)
		if name == nil {
			return
		}
		out = append(out, Symbol{Name: name.Text(d.Source), Kind: kind, Range: nodeRange(n)})
	})
	return out
}

func walk(n *gotreesitter.Node, visit func(*gotreesitter.Node)) {
	if n == nil {
		return
	}
	visit(n)
	for i := 0; i < n.NamedChildCount(); i++ {
		walk(n.NamedChild(i), visit)
	}
}
func nodeRange(n *gotreesitter.Node) Range {
	a, b := n.StartPoint(), n.EndPoint()
	return Range{Start: Position{a.Row, a.Column}, End: Position{b.Row, b.Column}}
}
