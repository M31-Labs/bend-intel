package intel

import (
	"sort"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// CallSite is a syntax-derived call edge. It is deliberately labelled as
// structural: overloads, imports, and higher-order values are semantic
// questions for the Bend compiler.
type CallSite struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
	Kind   string `json:"kind"`
	Range  Range  `json:"range"`
}

// ParallelRegion describes a conservative opportunity in Bend's explicit
// branch/tuple constructs. It never predicts runtime scheduling or speedups.
type ParallelRegion struct {
	Kind        string   `json:"kind"`
	Range       Range    `json:"range"`
	Branches    []string `json:"branches"`
	Calls       []string `json:"calls,omitempty"`
	Confidence  string   `json:"confidence"`
	Explanation string   `json:"explanation"`
}

type BindingInfo struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Range       Range  `json:"range"`
	Scope       string `json:"scope"`
	ScopeID     int    `json:"scopeId"`
	Explanation string `json:"explanation"`
}

type PatternFinding struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Range   Range  `json:"range"`
}

// IdentifierNameAt exposes the syntax-safe cursor lookup needed by protocol
// adapters without leaking the parser's language handle.
func (d *Document) IdentifierNameAt(position Position) string {
	node := identifierAt(d, position)
	if node == nil {
		return ""
	}
	return node.Text(d.Source)
}

// CallSites extracts direct call/application edges and keeps the enclosing
// definition as the caller. Nested higher-order terms are represented as
// separate edges when their callee is syntactically known.
func (d *Document) CallSites() []CallSite {
	if d == nil || d.Tree == nil {
		return nil
	}
	var out []CallSite
	walk(d.Tree.RootNode(), func(node *gotreesitter.Node) {
		typ := node.Type(d.language)
		if typ != "call_expression" && typ != "fun_application" {
			return
		}
		callee := directCallee(node, d.language, d.Source)
		if callee == "" {
			return
		}
		caller := enclosingDefinitionName(node, d.language, d.Source)
		out = append(out, CallSite{Caller: caller, Callee: callee, Kind: typ, Range: nodeRange(node)})
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Range.Start.Line == out[j].Range.Start.Line {
			return out[i].Range.Start.Character < out[j].Range.Start.Character
		}
		return out[i].Range.Start.Line < out[j].Range.Start.Line
	})
	return out
}

func directCallee(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) string {
	if node == nil {
		return ""
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "identifier" {
			return child.Text(source)
		}
		// An application can have a parenthesized callee. Do not walk into
		// arguments before finding the first function expression.
		if name := firstIdentifier(child, lang, source); name != "" {
			return name
		}
		break
	}
	return ""
}

func firstIdentifier(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) string {
	if node == nil {
		return ""
	}
	if node.Type(lang) == "identifier" {
		return node.Text(source)
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		if name := firstIdentifier(node.NamedChild(i), lang, source); name != "" {
			return name
		}
	}
	return ""
}

func enclosingDefinitionName(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) string {
	for current := node; current != nil; current = current.Parent() {
		typ := current.Type(lang)
		if typ != "imp_function_definition" && typ != "fun_function_definition" && typ != "hvm_definition" {
			continue
		}
		if name := current.ChildByFieldName("name", lang); name != nil {
			return name.Text(source)
		}
	}
	return "<module>"
}

// ParallelStructure reports explicit branch containers whose calls could be
// independent. The implementation prefers Bend's bend/when/else nodes and
// falls back to tuple/superposition branches; it does not infer data races.
func (d *Document) ParallelStructure() []ParallelRegion {
	if d == nil || d.Tree == nil {
		return nil
	}
	var out []ParallelRegion
	walk(d.Tree.RootNode(), func(node *gotreesitter.Node) {
		typ := node.Type(d.language)
		if typ != "bend_statement" && typ != "fun_bend" && typ != "tuple" && typ != "superposition" {
			return
		}
		branches := branchChildren(node, d.language)
		if len(branches) < 2 {
			return
		}
		calls := make([]string, 0)
		seen := map[string]bool{}
		for _, branch := range branches {
			for _, call := range callsInNode(d, branch) {
				if !seen[call.Callee] {
					seen[call.Callee] = true
					calls = append(calls, call.Callee)
				}
			}
		}
		labels := make([]string, len(branches))
		for i, branch := range branches {
			labels[i] = branch.Type(d.language)
		}
		out = append(out, ParallelRegion{Kind: typ, Range: nodeRange(node), Branches: labels, Calls: calls, Confidence: "structural", Explanation: "These explicit Bend branches have no direct syntactic ordering edge; compiler/runtime effects are not inferred."})
	})
	return out
}

func branchChildren(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	var branches []*gotreesitter.Node
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		typ := child.Type(lang)
		if typ == "when_clause" || typ == "else_clause" || typ == "match_case" || typ == "switch_case" || typ == "tuple" || typ == "superposition" {
			branches = append(branches, child)
		}
	}
	return branches
}

func callsInNode(d *Document, node *gotreesitter.Node) []CallSite {
	if node == nil {
		return nil
	}
	var out []CallSite
	for _, call := range d.CallSites() {
		if call.Range.Start.Line < node.StartPoint().Row || call.Range.End.Line > node.EndPoint().Row {
			continue
		}
		// Point filtering is sufficient for normal source ranges and avoids
		// reaching into an adjacent branch on the same line.
		if call.Range.Start.Line == node.StartPoint().Row && call.Range.Start.Character < node.StartPoint().Column || call.Range.End.Line == node.EndPoint().Row && call.Range.End.Character > node.EndPoint().Column {
			continue
		}
		out = append(out, call)
	}
	return out
}

func (d *Document) BindingInfo(position Position) *BindingInfo {
	node := identifierAt(d, position)
	if node == nil {
		return nil
	}
	name := node.Text(d.Source)
	binding := d.Scopes().bindingAt(node, name)
	if binding == nil {
		return &BindingInfo{Name: name, Kind: "unresolved", Range: nodeRange(node), Scope: "unknown", Explanation: "No lexical binding was found; imports and compiler semantics may still resolve this name."}
	}
	scope := "unknown"
	for _, candidate := range d.Scopes().Scopes {
		if candidate.ID == binding.ScopeID {
			scope = string(candidate.Kind)
			break
		}
	}
	return &BindingInfo{Name: binding.Name, Kind: binding.Kind, Range: binding.Range, Scope: scope, ScopeID: binding.ScopeID, Explanation: "Resolved by the Bend syntax scope graph; compiler-level imports and unscoped variables remain authoritative."}
}

func (d *Document) PatternCoverage() []PatternFinding {
	if d == nil || d.Tree == nil {
		return nil
	}
	var out []PatternFinding
	walk(d.Tree.RootNode(), func(node *gotreesitter.Node) {
		typ := node.Type(d.language)
		if typ != "match_statement" && typ != "switch_statement" && typ != "fun_match" && typ != "fun_switch" {
			return
		}
		var cases []*gotreesitter.Node
		walk(node, func(child *gotreesitter.Node) {
			if child != node && (child.Type(d.language) == "match_case" || child.Type(d.language) == "switch_case" || child.Type(d.language) == "_fun_match_case") {
				cases = append(cases, child)
			}
		})
		seen := map[string]*gotreesitter.Node{}
		wildcard := false
		for _, caseNode := range cases {
			pattern := firstPatternChild(caseNode, d.language)
			if pattern == nil {
				continue
			}
			text := strings.TrimSpace(pattern.Text(d.Source))
			if previous := seen[text]; previous != nil {
				out = append(out, PatternFinding{Kind: "duplicate", Message: "Duplicate pattern is unreachable after the earlier arm.", Range: nodeRange(pattern)})
			}
			seen[text] = pattern
			if wildcard {
				out = append(out, PatternFinding{Kind: "unreachable", Message: "This arm follows a wildcard pattern and is structurally unreachable.", Range: nodeRange(caseNode)})
			}
			if text == "_" || text == "*" {
				wildcard = true
			}
		}
	})
	return out
}

func firstPatternChild(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		typ := child.Type(lang)
		if strings.Contains(typ, "pattern") || typ == "identifier" || typ == "integer" {
			return child
		}
	}
	return nil
}

// HVMView is intentionally a structural view, not a claim that Tree-sitter
// performed Bend's desugaring. A compiler sidecar can replace it with real
// generated HVM when that protocol is available.
func (d *Document) HVMView() string {
	if d == nil || d.Tree == nil {
		return ""
	}
	return d.Tree.RootNode().SExpr(d.language)
}
