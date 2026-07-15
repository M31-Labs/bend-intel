package bendlang

import "testing"

func FuzzBendParserDoesNotPanic(f *testing.F) {
	for _, seed := range []string{
		"",
		"def add(x, y):\n  return x + y\n",
		"def main():\n  if 1:\n    return λx x\n",
		"type Option:\n  None\n  Some {value}\n",
		"def incomplete(:\n  return\n",
		"# unicode λ and indentation\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		parser, err := NewParser()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.Parse(source); err != nil {
			t.Fatalf("parser failed on recoverable input: %v", err)
		}
	})
}
