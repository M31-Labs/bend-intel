package intel

import "testing"

func TestIncrementalTreeEqualsCleanTree(t *testing.T) {
	tests := []struct {
		name, source string
		change       TextChange
	}{
		{"identifier", "def old(x):\n  return x\n", TextChange{Range: &Range{Start: Position{0, 4}, End: Position{0, 7}}, Text: "new"}},
		{"indentation", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{1, 0}, End: Position{1, 2}}, Text: "    "}},
		{"comment", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{1, 10}, End: Position{1, 10}}, Text: " # value"}},
		{"unicode", "def λfoo(x):\n  return x\n", TextChange{Range: &Range{Start: Position{0, 5}, End: Position{0, 8}}, Text: "bar"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if err := doc.ApplyChanges([]TextChange{test.change}); err != nil {
				t.Fatal(err)
			}
			clean, err := Parse(doc.Source)
			if err != nil {
				t.Fatal(err)
			}
			incremental := doc.Tree.RootNode().SExpr(doc.language)
			fresh := clean.Tree.RootNode().SExpr(clean.language)
			if incremental != fresh {
				t.Fatalf("incremental tree differs from clean tree\nincremental: %s\nclean: %s", incremental, fresh)
			}
		})
	}
}

func TestApplyUTF16Change(t *testing.T) {
	doc, err := Parse([]byte("def λfoo(x):\n  return x\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = doc.ApplyChanges([]TextChange{{Range: &Range{Start: Position{0, 5}, End: Position{0, 8}}, Text: "bar"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(doc.Source); got != "def λbar(x):\n  return x\n" {
		t.Fatalf("source = %q", got)
	}
}
