package intel

import "testing"

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
