package intel

import (
	_ "embed"
	"testing"

	"github.com/M31-Labs/bend-intel/bendlang"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

//go:embed testdata/radix_prefix.bend
var radixPrefix []byte

func TestOutlineImperativeFunction(t *testing.T) {
	doc, err := Parse([]byte("def add(x, y):\n  return x + y\n"))
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := doc.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	symbols := doc.Symbols()
	if len(symbols) != 1 || symbols[0].Name != "add" || symbols[0].Kind != "function" {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func TestMalformedSourceProducesDiagnostic(t *testing.T) {
	doc, err := Parse([]byte("def broken(:\n  return\n"))
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := doc.Diagnostics(); len(diagnostics) == 0 {
		t.Fatal("expected syntax diagnostic")
	}
}

func TestScopeAwareParameterDefinition(t *testing.T) {
	doc, err := Parse([]byte("def first(x):\n  return x\n\ndef second(x):\n  return x\n"))
	if err != nil {
		t.Fatal(err)
	}
	definition := doc.Definition(Position{Line: 1, Character: 9})
	if definition == nil || definition.Kind != "parameter" || definition.Range.Start.Line != 0 {
		t.Fatalf("definition = %#v", definition)
	}
	second := doc.Definition(Position{Line: 4, Character: 9})
	if second == nil || second.Kind != "parameter" || second.Range.Start.Line != 3 {
		t.Fatalf("second definition = %#v", second)
	}
}

func TestSelectionRangesGrowFromIdentifierToRoot(t *testing.T) {
	doc, err := Parse([]byte("def add(x):\n  return x + 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	ranges := doc.SelectionRanges([]Position{{Line: 1, Character: 9}})
	if len(ranges) != 1 || ranges[0].Parent == nil {
		t.Fatalf("selection ranges = %#v", ranges)
	}
	if ranges[0].Range.Start.Line != 1 {
		t.Fatalf("selection hierarchy = %#v", ranges[0])
	}
	last := ranges[0].Parent
	for last.Parent != nil {
		last = last.Parent
	}
	if last.Range.Start.Line != 0 {
		t.Fatalf("selection did not reach root = %#v", ranges[0])
	}
}

func TestScopeGraphIncludesLambdaParameters(t *testing.T) {
	doc, err := Parse([]byte("def main():\n  return lambda x: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	definition := doc.Definition(Position{Line: 1, Character: 19})
	if definition == nil || definition.Kind != "parameter" {
		t.Fatalf("lambda definition = %#v, diagnostics=%#v", definition, doc.Diagnostics())
	}
}

func TestScopeGraphIncludesMatchArmBindings(t *testing.T) {
	doc, err := Parse([]byte("def main(value):\n  match value:\n    case item:\n      return item\n"))
	if err != nil {
		t.Fatal(err)
	}
	definition := doc.Definition(Position{Line: 3, Character: 13})
	if definition == nil || definition.Kind != "variable" {
		t.Fatalf("match binding = %#v, diagnostics=%#v", definition, doc.Diagnostics())
	}
}

func TestTypedHeaderParsesWithoutRecovery(t *testing.T) {
	source := []byte("def main(x: u24) -> IO(u24):\n  return x\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := doc.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("typed header diagnostics = %#v", diagnostics)
	}
	symbols := doc.Symbols()
	if len(symbols) != 1 || symbols[0].Name != "main" {
		t.Fatalf("typed header symbols = %#v", symbols)
	}
	if got := string(doc.Source); got != string(source) {
		t.Fatalf("recovery changed source = %q", got)
	}
	if doc.Recovered() {
		t.Fatal("current typed header should parse without structural recovery")
	}
}

func TestLexicalOutlineRecoversCurrentFunctionalDefinitions(t *testing.T) {
	source := []byte("type Tree(t):\n  Leaf\n\n(Part) : (List u24) -> (List u24) = List/Nil\n\n(Part List/Nil) = List/Nil\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	symbols := doc.Symbols()
	if len(symbols) < 3 {
		t.Fatalf("symbols = %#v", symbols)
	}
	if symbols[0].Name != "Tree" || symbols[1].Name != "Part" || symbols[2].Name != "Part" {
		t.Fatalf("functional outline = %#v", symbols)
	}
}

func TestCurrentTypedBendRecoveryKeepsLaterDefinitions(t *testing.T) {
	source := []byte("type Tree(t):\n  Node { val: t, ~left: Tree(t), ~right: Tree(t) }\n  Leaf\n\ndef gen(depth: u24) -> Tree(u24):\n  bend height=0, val=1:\n    when height < depth:\n      tree = Tree/Node { val: val, left: fork(height+1, 2*val), right: fork(height+1, 2*val+1) }\n    else:\n      tree = Tree/Leaf\n  return tree\n\n# next top-level definition\ndef main() -> u24:\n  return gen(1)")
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, symbol := range doc.Symbols() {
		names[symbol.Name] = true
	}
	if !names["Tree"] || !names["gen"] || !names["main"] {
		t.Fatalf("recovered symbols = %#v", doc.Symbols())
	}
	if diagnostics := doc.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("recovered diagnostics = %#v", diagnostics)
	}
}

func TestParseHealthCatchesSilentNoStacksPrefix(t *testing.T) {
	source := append([]byte(nil), radixPrefix...)
	parser, err := bendlang.NewParser()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := parser.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if raw.ParseStopReason() == gotreesitter.ParseStopAccepted && !raw.RootNode().HasError() {
		t.Fatalf("raw parser unexpectedly accepted hidden failure: stop=%s tree=%s", raw.ParseStopReason(), raw.RootNode().SExpr(raw.Language()))
	}
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Complete() || !doc.Recovered() || doc.Health().Stopped {
		t.Fatalf("recovery health = %#v", doc.Health())
	}
	if diagnostics := doc.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("recovered diagnostics = %#v", diagnostics)
	}
}

func TestIncompleteStructuralTreePublishesParserDiagnostic(t *testing.T) {
	source := []byte("def f(x: u24) -> u24:\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Complete() {
		t.Fatalf("fixture unexpectedly complete: %#v", doc.Health())
	}
	diagnostics := doc.Diagnostics()
	if len(diagnostics) == 0 {
		t.Fatalf("diagnostics = %#v, health = %#v", diagnostics, doc.Health())
	}
}
