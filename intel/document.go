package intel

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/M31-Labs/bend-intel/bendlang"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Diagnostic struct {
	Message  string `json:"message"`
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
}
type Symbol struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Range Range  `json:"range"`
}
type FoldingRange struct {
	StartLine uint32 `json:"startLine"`
	EndLine   uint32 `json:"endLine"`
}

type Document struct {
	Source   []byte
	Tree     *gotreesitter.Tree
	language *gotreesitter.Language
}

// Offset converts an LSP UTF-16 position to a UTF-8 byte offset.
func (d *Document) Offset(position Position) (int, bool) {
	line, offset := uint32(0), 0
	for line < position.Line && offset < len(d.Source) {
		if d.Source[offset] == '\n' {
			line++
		}
		offset++
	}
	if line != position.Line {
		return 0, false
	}
	units := uint32(0)
	for offset < len(d.Source) && d.Source[offset] != '\n' {
		if units == position.Character {
			return offset, true
		}
		r, size := utf8.DecodeRune(d.Source[offset:])
		width := uint32(len(utf16.Encode([]rune{r})))
		if units+width > position.Character {
			return 0, false
		}
		units += width
		offset += size
	}
	return offset, units == position.Character
}

func (d *Document) NodeAt(position Position) *gotreesitter.Node {
	offset, ok := d.Offset(position)
	if !ok {
		return nil
	}
	if offset == len(d.Source) && offset > 0 {
		offset--
	}
	return d.Tree.RootNode().DescendantForByteRange(uint32(offset), uint32(offset+1))
}

func (d *Document) Definition(position Position) *Symbol {
	node := d.NodeAt(position)
	if node == nil {
		return nil
	}
	for node != nil && node.Type(d.language) != "identifier" {
		node = node.Parent()
	}
	if node == nil {
		return nil
	}
	name := node.Text(d.Source)
	for _, symbol := range d.Symbols() {
		if symbol.Name == name {
			copy := symbol
			return &copy
		}
	}
	return nil
}

func (d *Document) References(position Position) []Range {
	node := d.NodeAt(position)
	if node == nil {
		return nil
	}
	for node != nil && node.Type(d.language) != "identifier" {
		node = node.Parent()
	}
	if node == nil {
		return nil
	}
	name := node.Text(d.Source)
	var out []Range
	walk(d.Tree.RootNode(), func(candidate *gotreesitter.Node) {
		if candidate.Type(d.language) == "identifier" && candidate.Text(d.Source) == name {
			out = append(out, nodeRange(candidate))
		}
	})
	return out
}

func (d *Document) FoldingRanges() []FoldingRange {
	foldable := map[string]bool{"imp_function_definition": true, "fun_function_definition": true, "hvm_definition": true, "imp_type_definition": true, "fun_type_definition": true, "object_definition": true, "match_statement": true, "switch_statement": true, "bend_statement": true, "fold_statement": true}
	var out []FoldingRange
	walk(d.Tree.RootNode(), func(n *gotreesitter.Node) {
		a, b := n.StartPoint(), n.EndPoint()
		if foldable[n.Type(d.language)] && b.Row > a.Row {
			out = append(out, FoldingRange{a.Row, b.Row})
		}
	})
	return out
}

func (d *Document) Hover(position Position) string {
	node := d.NodeAt(position)
	if node == nil {
		return ""
	}
	if def := d.Definition(position); def != nil {
		return fmt.Sprintf("**%s** `%s`", def.Kind, def.Name)
	}
	return fmt.Sprintf("Bend syntax: `%s`", node.Type(d.language))
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
		*out = append(*out, Diagnostic{Message: "unexpected Bend syntax", Range: nodeRange(n), Severity: 1, Source: "bend-syntax"})
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
