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
