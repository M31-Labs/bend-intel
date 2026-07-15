package intel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportsWorkspaceSymbolsAndRename(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.bend"), []byte("import lib/defs\ndef main():\n  return add(1, 2)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "lib"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib", "defs.bend"), []byte("def add(x, y):\n  return x + y\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := NewWorkspace(root)
	if err := w.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.Documents) != 2 {
		t.Fatalf("documents = %d", len(w.Documents))
	}
	var mainURI string
	for uri := range w.Documents {
		if filepath.Base(uri) == "main.bend" {
			mainURI = uri
		}
	}
	if mainURI == "" {
		t.Fatal("main document was not indexed")
	}
	imports := w.Documents[mainURI].Imports()
	if len(imports) != 1 || imports[0].Path != "lib/defs" {
		t.Fatalf("imports = %#v", imports)
	}
	if len(w.Symbols()) != 2 {
		t.Fatalf("workspace symbols = %#v", w.Symbols())
	}
	completions := w.Completions(mainURI, Position{})
	if len(completions) < 10 {
		t.Fatalf("completion count = %d", len(completions))
	}
	edit, err := w.Rename(mainURI, Position{Line: 2, Character: 9}, "sum")
	if err != nil {
		t.Fatal(err)
	}
	if len(edit) != 2 || edit[0].NewText != "sum" || edit[1].NewText != "sum" {
		t.Fatalf("rename edits = %#v", edit)
	}
}
