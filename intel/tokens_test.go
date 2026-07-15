package intel

import "testing"

func TestSemanticTokensFromQuery(t *testing.T) {
	doc, err := Parse([]byte("def add(x):\n  return x + 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := doc.SemanticTokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("semantic token data = %v", data)
	}
}
