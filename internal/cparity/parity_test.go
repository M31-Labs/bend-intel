//go:build cgo && parity

package cparity

import "testing"

func TestCAndGoTreesAgreeOnFixtures(t *testing.T) {
	fixtures := []string{
		"",
		"def add(x, y):\n  return x + y\n",
		"def main():\n  if 1 == 1:\n    return 1\n  else:\n    return 0\n",
		"import lib/defs\n",
		"type Option:\n  None\n  Some {value}\n",
		"main = (λk λs λz let {s0 s1} = s; (s0 ((k s1) z)) λq λw w)",
		"main = let {x1 x2} = ($a $b); λy let {y1 y2} = y; (λ$a (y1 x1) λ$b (y2 x2))",
		"main = let fst = (λt let (f, *) = t; f); let snd = (λt let (*, s) = t; s); (snd (fst ((1, 3), 2)))",
	}
	for _, source := range fixtures {
		result, err := Compare([]byte(source))
		if err != nil {
			t.Fatalf("source %q: %v", source, err)
		}
		if !result.Equal {
			t.Fatalf("C and Go trees differ for %q\nC: %s\nGo: %s", source, result.CTree, result.GoTree)
		}
	}
}
