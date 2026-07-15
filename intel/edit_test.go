package intel

import "testing"

func TestIncrementalTreeEqualsCleanTree(t *testing.T) {
	tests := []struct {
		name, source string
		change       TextChange
	}{
		{"identifier", "def old(x):\n  return x\n", TextChange{Range: &Range{Start: Position{0, 4}, End: Position{0, 7}}, Text: "new"}},
		{"identifier insertion", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{1, 10}, End: Position{1, 10}}, Text: " + 2"}},
		{"identifier deletion", "def main():\n  return value\n", TextChange{Range: &Range{Start: Position{1, 9}, End: Position{1, 14}}, Text: ""}},
		{"indentation", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{1, 0}, End: Position{1, 2}}, Text: "    "}},
		{"dedentation", "def main():\n    return 1\n", TextChange{Range: &Range{Start: Position{1, 0}, End: Position{1, 4}}, Text: "  "}},
		{"new block", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{2, 0}, End: Position{2, 0}}, Text: "\ndef other():\n  return 2\n"}},
		{"block removal", "def main():\n  return 1\n\ndef other():\n  return 2\n", TextChange{Range: &Range{Start: Position{2, 0}, End: Position{5, 0}}, Text: ""}},
		{"comment", "def main():\n  return 1\n", TextChange{Range: &Range{Start: Position{1, 10}, End: Position{1, 10}}, Text: " # value"}},
		{"operator replacement", "def main():\n  return 1 + 2\n", TextChange{Range: &Range{Start: Position{1, 11}, End: Position{1, 12}}, Text: "*"}},
		{"unclosed delimiter", "def main():\n  return (1)\n", TextChange{Range: &Range{Start: Position{1, 11}, End: Position{1, 12}}, Text: ""}},
		{"complete invalid syntax", "def main():\n  return (1\n", TextChange{Range: &Range{Start: Position{1, 11}, End: Position{1, 11}}, Text: ")"}},
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
