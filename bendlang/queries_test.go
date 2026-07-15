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
