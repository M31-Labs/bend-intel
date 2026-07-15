package intel

import (
	"bytes"
	"fmt"
	"sort"
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
type SelectionRange struct {
	Range  Range           `json:"range"`
	Parent *SelectionRange `json:"parent,omitempty"`
}

type Document struct {
	Source     []byte
	Tree       *gotreesitter.Tree
	language   *gotreesitter.Language
	treeSource []byte
	scopeGraph *ScopeGraph
}

// ParseHealth describes the structural parser result independently of syntax
// ERROR nodes. gotreesitter intentionally returns partial trees for editor
// workloads, so callers must inspect the stop reason and covered byte range
// before treating a tree as a complete parse.
type ParseHealth struct {
	Complete     bool   `json:"complete"`
	Recovered    bool   `json:"recovered"`
	RootHasError bool   `json:"rootHasError"`
	Stopped      bool   `json:"stopped"`
	StopReason   string `json:"stopReason"`
	EndByte      uint32 `json:"endByte"`
	SourceBytes  int    `json:"sourceBytes"`
}

// Recovered reports whether the structural tree was parsed from a
// range-preserving recovery view rather than the exact source bytes. The
// original source remains in Source and must be used for all editor ranges.
func (d *Document) Recovered() bool {
	return !bytes.Equal(d.Source, d.treeSource) || !d.Complete()
}

// Complete reports whether the current CST accepted the whole source without
// hidden root errors or a partial-parser stop. It is intentionally stricter
// than checking named ERROR nodes; a no_stacks_alive prefix can otherwise look
// clean to a tree walk.
func (d *Document) Complete() bool {
	if d == nil || d.Tree == nil || d.Tree.RootNode() == nil {
		return false
	}
	root := d.Tree.RootNode()
	return !root.HasError() && d.Tree.ParseStopReason() == gotreesitter.ParseStopAccepted && int(root.EndByte()) >= len(d.Source)
}

// Health returns a stable, editor-facing parse status for telemetry, corpus
// reports, and clients that want to distinguish exact syntax from recovery.
func (d *Document) Health() ParseHealth {
	if d == nil || d.Tree == nil || d.Tree.RootNode() == nil {
		sourceBytes := 0
		if d != nil {
			sourceBytes = len(d.Source)
		}
		return ParseHealth{Recovered: true, Stopped: true, StopReason: "nil-tree", SourceBytes: sourceBytes}
	}
	root := d.Tree.RootNode()
	stop := d.Tree.ParseStopReason()
	return ParseHealth{
		Complete:     d.Complete(),
		Recovered:    d.Recovered(),
		RootHasError: root.HasError(),
		Stopped:      stop != gotreesitter.ParseStopAccepted,
		StopReason:   string(stop),
		EndByte:      root.EndByte(),
		SourceBytes:  len(d.Source),
	}
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
	if binding := d.Scopes().bindingAt(node, name); binding != nil {
		return &Symbol{Name: binding.Name, Kind: binding.Kind, Range: binding.Range}
	}
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
	target := d.Scopes().bindingAt(node, name)
	var out []Range
	walk(d.Tree.RootNode(), func(candidate *gotreesitter.Node) {
		if candidate.Type(d.language) == "identifier" && candidate.Text(d.Source) == name && (target == nil || sameBinding(target, d.Scopes().bindingAt(candidate, name))) {
			out = append(out, nodeRange(candidate))
		}
	})
	return out
}

func sameBinding(a, b *Binding) bool {
	return a != nil && b != nil && a.Name == b.Name && a.Kind == b.Kind && a.ScopeID == b.ScopeID && a.Range == b.Range
}

func (d *Document) SelectionRanges(positions []Position) []SelectionRange {
	out := make([]SelectionRange, 0, len(positions))
	for _, position := range positions {
		node := d.NodeAt(position)
		if node == nil {
			out = append(out, SelectionRange{})
			continue
		}
		var chain *SelectionRange
		for current := node; current != nil; current = current.Parent() {
			chain = &SelectionRange{Range: nodeRange(current), Parent: chain}
		}
		var ordered *SelectionRange
		for current := chain; current != nil; current = current.Parent {
			ordered = &SelectionRange{Range: current.Range, Parent: ordered}
		}
		out = append(out, *ordered)
	}
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
	tree, treeSource, err := parseWithRecovery(parser, lang, source)
	if err != nil {
		return nil, fmt.Errorf("parse Bend: %w", err)
	}
	return &Document{Source: append([]byte(nil), source...), Tree: tree, language: lang, treeSource: treeSource}, nil
}

func (d *Document) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, 0)
	collectInnermostErrors(d.Tree.RootNode(), &out)
	// A parser can stop without materializing a named ERROR node. Report that
	// condition explicitly so LSP clients never mistake a truncated structural
	// tree for a clean document. Recovery candidates that cover the full source
	// and have accepted stop metadata do not reach this branch.
	if len(out) == 0 && !d.Complete() {
		root := d.Tree.RootNode()
		end := root.EndPoint()
		message := "Bend parser did not accept the complete source"
		if reason := d.Tree.ParseStopReason(); reason != gotreesitter.ParseStopAccepted {
			message = fmt.Sprintf("Bend parser stopped before the end of the source (%s); structural features may be partial", reason)
		} else if root.HasError() {
			message = "Bend parser returned a structural error tree; structural features may be partial"
		}
		out = append(out, Diagnostic{Message: message, Range: Range{Start: Position{end.Row, end.Column}, End: Position{end.Row, end.Column}}, Severity: 2, Source: "bend-parser"})
	}
	seenLines := make(map[uint32]bool, len(out))
	for _, diagnostic := range out {
		seenLines[diagnostic.Range.Start.Line] = true
	}
	if len(out) == 0 {
		return out
	}
	for _, line := range missingDefinitionLines(d.Source, d.Tree.RootNode(), d.language) {
		if seenLines[line] {
			continue
		}
		start, end := lineBounds(d.Source, line)
		out = append(out, Diagnostic{Message: "Bend definition is outside the current grammar baseline", Range: Range{Start: Position{Line: line}, End: Position{Line: line, Character: uint32(end - start)}}, Severity: 2, Source: "bend-syntax"})
	}
	return out
}

func lineBounds(source []byte, line uint32) (int, int) {
	start := 0
	for current := uint32(0); current < line && start < len(source); start++ {
		if source[start] == '\n' {
			current++
		}
	}
	end := start
	for end < len(source) && source[end] != '\n' {
		end++
	}
	return start, end
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
	out = append(out, lexicalSymbols(d.Source, out)...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Range.Start.Line == out[j].Range.Start.Line {
			return out[i].Range.Start.Character < out[j].Range.Start.Character
		}
		return out[i].Range.Start.Line < out[j].Range.Start.Line
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
