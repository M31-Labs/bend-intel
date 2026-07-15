package bendlang

import "testing"

func TestScannerSerializationRoundTrip(t *testing.T) {
	s := externalScanner{}
	original := &scannerState{indents: []uint16{0, 2, 4}}
	buf := make([]byte, 16)
	n := s.Serialize(original, buf)
	restored := &scannerState{}
	s.Deserialize(restored, buf[:n])
	if len(restored.indents) != 3 || restored.indents[1] != 2 || restored.indents[2] != 4 {
		t.Fatalf("restored indents = %v", restored.indents)
	}
}

func TestHighlightsQueryCompiles(t *testing.T) {
	if _, err := Highlights(); err != nil {
		t.Fatal(err)
	}
}
