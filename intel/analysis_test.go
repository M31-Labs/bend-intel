package intel

import "testing"

func TestAnalysisFindsCallsBindingsAndPatterns(t *testing.T) {
	doc, err := Parse([]byte("def add(x, y):\n  return x + y\n\ndef main(v):\n  match v:\n    case _:\n      return add(1, 2)\n    case _:\n      return add(3, 4)\n"))
	if err != nil {
		t.Fatal(err)
	}
	calls := doc.CallSites()
	if len(calls) != 2 || calls[0].Callee != "add" || calls[1].Callee != "add" {
		t.Fatalf("calls = %#v", calls)
	}
	info := doc.BindingInfo(Position{Line: 1, Character: 9})
	if info == nil || info.Name != "x" || info.Kind != "parameter" {
		t.Fatalf("binding = %#v", info)
	}
	findings := doc.PatternCoverage()
	if len(findings) == 0 || findings[0].Kind != "duplicate" {
		t.Fatalf("pattern findings = %#v", findings)
	}
	if len(doc.HVMView()) == 0 {
		t.Fatal("expected structural view")
	}
}

func TestParallelStructureIsConservative(t *testing.T) {
	doc, err := Parse([]byte("def main(x):\n  bend x:\n    when x > 0:\n      return add(x, 1)\n    else:\n      return mul(x, 2)\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, region := range doc.ParallelStructure() {
		if region.Confidence != "structural" || region.Explanation == "" {
			t.Fatalf("region = %#v", region)
		}
	}
}

func TestWorkspaceCallHierarchyKeepsCallerAndCallee(t *testing.T) {
	w := NewWorkspace("")
	mainURI := "file:///main.bend"
	if err := w.Add(mainURI, []byte("def add(x):\n  return x\n\ndef main():\n  return add(1)\n")); err != nil {
		t.Fatal(err)
	}
	incoming, outgoing := w.CallHierarchy(mainURI, Position{Line: 0, Character: 4})
	if len(incoming) != 1 || incoming[0].Caller != "main" || incoming[0].Callee != "add" {
		t.Fatalf("incoming = %#v", incoming)
	}
	_, outgoing = w.CallHierarchy(mainURI, Position{Line: 3, Character: 4})
	if len(outgoing) != 1 || outgoing[0].Caller != "main" || outgoing[0].Callee != "add" {
		t.Fatalf("outgoing = %#v", outgoing)
	}
}
