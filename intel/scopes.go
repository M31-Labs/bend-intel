package intel

import "github.com/odvcencio/gotreesitter"

type ScopeKind string

const (
	ScopeFile     ScopeKind = "file"
	ScopeFunction ScopeKind = "function"
	ScopeLambda   ScopeKind = "lambda"
	ScopeMatch    ScopeKind = "match-arm"
	ScopeSwitch   ScopeKind = "switch-arm"
	ScopeFold     ScopeKind = "fold-arm"
	ScopeBend     ScopeKind = "bend"
)

type Scope struct {
	ID     int       `json:"id"`
	Kind   ScopeKind `json:"kind"`
	Range  Range     `json:"range"`
	Parent int       `json:"parent"`
}

type Binding struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Range   Range  `json:"range"`
	ScopeID int    `json:"scope"`
}

type ScopeGraph struct {
	Scopes   []Scope
	Bindings []Binding
}

func (d *Document) Scopes() *ScopeGraph {
	if d.scopeGraph != nil {
		return d.scopeGraph
	}
	graph := &ScopeGraph{Scopes: []Scope{{ID: 0, Kind: ScopeFile, Range: nodeRange(d.Tree.RootNode()), Parent: -1}}}
	buildScopes(d.Tree.RootNode(), 0, graph, d.language, d.Source)
	d.scopeGraph = graph
	return graph
}

func buildScopes(node *gotreesitter.Node, current int, graph *ScopeGraph, lang *gotreesitter.Language, source []byte) {
	if node == nil {
		return
	}
	typ := node.Type(lang)
	if typ == "imp_function_definition" || typ == "fun_function_definition" {
		name := node.ChildByFieldName("name", lang)
		if name != nil {
			graph.Bindings = append(graph.Bindings, Binding{Name: name.Text(source), Kind: "function", Range: nodeRange(name), ScopeID: current})
		}
		childScope := addScope(graph, ScopeFunction, nodeRange(node), current)
		if parameters := node.ChildByFieldName("parameters", lang); parameters != nil {
			addParameterBindings(parameters, childScope, graph, lang, source)
		}
		if typ == "fun_function_definition" {
			body := node.ChildByFieldName("body", lang)
			for i := 0; i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				if child == nil || child == name || child == body {
					continue
				}
				if child.Type(lang) == "pattern" {
					addPatternBindings(child, childScope, graph, lang, source)
				}
			}
		}
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil && child != name {
				buildScopes(child, childScope, graph, lang, source)
			}
		}
		return
	}
	if typ == "imp_lambda" || typ == "fun_lambda" {
		childScope := addScope(graph, ScopeLambda, nodeRange(node), current)
		parameters := node.ChildByFieldName("parameters", lang)
		if parameters == nil {
			parameters = firstNamedChildType(node, lang, "parameters")
		}
		if parameters != nil {
			addParameterBindings(parameters, childScope, graph, lang, source)
		}
		if pattern := node.ChildByFieldName("pattern", lang); pattern != nil {
			addPatternBindings(pattern, childScope, graph, lang, source)
		} else if pattern := firstNamedChildType(node, lang, "pattern"); pattern != nil {
			addPatternBindings(pattern, childScope, graph, lang, source)
		}
		for i := 0; i < node.NamedChildCount(); i++ {
			buildScopes(node.NamedChild(i), childScope, graph, lang, source)
		}
		return
	}
	if kind, ok := armScopeKind(typ); ok {
		childScope := addScope(graph, kind, nodeRange(node), current)
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child == nil {
				continue
			}
			if typ == "match_case" && child.Type(lang) == "match_pattern" {
				addPatternBindings(child, childScope, graph, lang, source)
			}
			buildScopes(child, childScope, graph, lang, source)
		}
		return
	}
	if typ == "assignment_statement" {
		if pattern := node.ChildByFieldName("pat", lang); pattern != nil {
			addPatternBindings(pattern, current, graph, lang, source)
		}
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		buildScopes(node.NamedChild(i), current, graph, lang, source)
	}
}

func armScopeKind(typ string) (ScopeKind, bool) {
	switch typ {
	case "match_case":
		return ScopeMatch, true
	case "switch_case":
		return ScopeSwitch, true
	case "fold_statement":
		return ScopeFold, true
	case "bend_statement":
		return ScopeBend, true
	default:
		return "", false
	}
}

func addScope(graph *ScopeGraph, kind ScopeKind, span Range, parent int) int {
	id := len(graph.Scopes)
	graph.Scopes = append(graph.Scopes, Scope{ID: id, Kind: kind, Range: span, Parent: parent})
	return id
}

func firstNamedChildType(node *gotreesitter.Node, lang *gotreesitter.Language, typ string) *gotreesitter.Node {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == typ {
			return child
		}
	}
	return nil
}

func addParameterBindings(node *gotreesitter.Node, scopeID int, graph *ScopeGraph, lang *gotreesitter.Language, source []byte) {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "identifier" {
			graph.Bindings = append(graph.Bindings, Binding{Name: child.Text(source), Kind: "parameter", Range: nodeRange(child), ScopeID: scopeID})
		}
	}
}

func addPatternBindings(node *gotreesitter.Node, scopeID int, graph *ScopeGraph, lang *gotreesitter.Language, source []byte) {
	if node.Type(lang) == "identifier" {
		graph.Bindings = append(graph.Bindings, Binding{Name: node.Text(source), Kind: "variable", Range: nodeRange(node), ScopeID: scopeID})
		return
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		addPatternBindings(node.NamedChild(i), scopeID, graph, lang, source)
	}
}

func (g *ScopeGraph) bindingAt(node *gotreesitter.Node, name string) *Binding {
	if node == nil {
		return nil
	}
	bestScope := 0
	start, end := node.StartPoint(), node.EndPoint()
	for _, scope := range g.Scopes {
		if positionLE(scope.Range.Start, Position{start.Row, start.Column}) && positionLE(Position{end.Row, end.Column}, scope.Range.End) {
			if bestScope == 0 || scopeSize(scope) <= scopeSize(g.Scopes[bestScope]) {
				bestScope = scope.ID
			}
		}
	}
	for scopeID := bestScope; scopeID >= 0; {
		for i := len(g.Bindings) - 1; i >= 0; i-- {
			binding := g.Bindings[i]
			if binding.ScopeID == scopeID && binding.Name == name && positionLE(binding.Range.Start, Position{start.Row, start.Column}) {
				copy := binding
				return &copy
			}
		}
		scopeID = g.Scopes[scopeID].Parent
	}
	return nil
}

func positionLE(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character <= b.Character
}

func scopeSize(scope Scope) uint64 {
	return uint64(scope.Range.End.Line-scope.Range.Start.Line)*1000000 + uint64(scope.Range.End.Character)
}
