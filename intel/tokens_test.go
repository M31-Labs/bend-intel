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

func TestSemanticTokensRetainCommentsDuringRecovery(t *testing.T) {
	doc, err := Parse([]byte("def main(x: u24) -> u24:\n  return x\n\n# keep this comment\ndef next() -> u24:\n  return 0"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := doc.SemanticTokens()
	if err != nil {
		t.Fatal(err)
	}
	for i := 3; i < len(data); i += 5 {
		if data[i] == 6 {
			return
		}
	}
	t.Fatalf("comment token missing from recovered stream: %v", data)
}
