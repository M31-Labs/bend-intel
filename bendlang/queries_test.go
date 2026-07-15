package bendlang

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

func TestQueriesCompileAgainstBendGrammar(t *testing.T) {
	lang, err := Language()
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"highlights": HighlightsQuery(),
		"locals":     LocalsQuery(),
		"tags":       TagsQuery(),
		"folds":      FoldsQuery(),
		"indents":    IndentsQuery(),
	} {
		if _, err := gotreesitter.NewQuery(source, lang); err != nil {
			t.Fatalf("%s query: %v", name, err)
		}
	}
}

func TestQueriesExecuteOnImperativeAndFunctionalTrees(t *testing.T) {
	source := []byte("def add(x, y):\n  return x + y\n\n(inc x) = (+ x 1)\n")
	parser, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	lang, err := Language()
	if err != nil {
		t.Fatal(err)
	}
	for name, querySource := range map[string]string{
		"highlights": HighlightsQuery(),
		"locals":     LocalsQuery(),
		"tags":       TagsQuery(),
		"folds":      FoldsQuery(),
		"indents":    IndentsQuery(),
	} {
		query, err := gotreesitter.NewQuery(querySource, lang)
		if err != nil {
			t.Fatalf("%s query: %v", name, err)
		}
		_ = query.Execute(tree)
	}
}

func TestFunctionalInlineLetFormsParseCleanly(t *testing.T) {
	fixtures := []string{
		"main = (λk λs λz let {s0 s1} = s; (s0 ((k s1) z)) λq λw w)",
		"main = let {x1 x2} = ($a $b); λy let {y1 y2} = y; (λ$a (y1 x1) λ$b (y2 x2))",
		"main = let fst = (λt let (f, *) = t; f); let snd = (λt let (*, s) = t; s); (snd (fst ((1, 3), 2)))",
	}
	for _, source := range fixtures {
		parser, err := NewParser()
		if err != nil {
			t.Fatal(err)
		}
		tree, err := parser.Parse([]byte(source))
		if err != nil {
			t.Fatalf("source %q: %v", source, err)
		}
		if tree.RootNode().HasError() || tree.ParseStopReason() != gotreesitter.ParseStopAccepted || int(tree.RootNode().EndByte()) != len(source) {
			t.Fatalf("functional source is not clean: stop=%s end=%d/%d tree=%s", tree.ParseStopReason(), tree.RootNode().EndByte(), len(source), tree.RootNode().SExpr(tree.Language()))
		}
	}
}
