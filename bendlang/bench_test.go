package bendlang

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

var benchmarkSource = []byte("def fib(n):\n  switch n:\n    case 0:\n      return 0\n    case _:\n      return fib(n - 1) + fib(n - 2)\n")

func BenchmarkParse(b *testing.B) {
	parser, err := NewParser()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkSource)))
	for i := 0; i < b.N; i++ {
		if _, err := parser.Parse(benchmarkSource); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIncrementalParse(b *testing.B) {
	parser, err := NewParser()
	if err != nil {
		b.Fatal(err)
	}
	source := append([]byte(nil), benchmarkSource...)
	tree, err := parser.Parse(source)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for i := 0; i < b.N; i++ {
		old := append([]byte(nil), source...)
		next := append([]byte(nil), source...)
		if next[len(next)-2] == '2' {
			next[len(next)-2] = '3'
		} else {
			next[len(next)-2] = '2'
		}
		tree.Edit(inputEditForReplace(len(next)-2, old, next))
		tree, err = parser.ParseIncremental(next, tree)
		if err != nil {
			b.Fatal(err)
		}
		source = next
	}
}

func inputEditForReplace(offset int, old, next []byte) gotreesitter.InputEdit {
	return gotreesitter.InputEdit{
		StartByte: uint32(offset), OldEndByte: uint32(offset + 1), NewEndByte: uint32(offset + 1),
		StartPoint: pointAt(old, offset), OldEndPoint: pointAt(old, offset+1), NewEndPoint: pointAt(next, offset+1),
	}
}

func pointAt(source []byte, offset int) gotreesitter.Point {
	if offset > len(source) {
		offset = len(source)
	}
	var point gotreesitter.Point
	for _, b := range source[:offset] {
		if b == '\n' {
			point.Row++
			point.Column = 0
		} else {
			point.Column++
		}
	}
	return point
}
