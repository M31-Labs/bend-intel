//go:build cgo && parity

package cparity

import "testing"

func TestCParserLoadsBendGrammar(t *testing.T) {
	got, err := Parse([]byte("def add(x, y):\n  return x + y\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("C parser returned an empty tree")
	}
}
