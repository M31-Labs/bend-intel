package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/M31-Labs/bend-intel/intel"
)

func main() {
	if len(os.Args) != 3 || (os.Args[1] != "check" && os.Args[1] != "outline") {
		fmt.Fprintln(os.Stderr, "usage: bend-intel <check|outline> <file.bend>")
		os.Exit(2)
	}
	source, err := os.ReadFile(os.Args[2])
	if err != nil {
		fail(err)
	}
	doc, err := intel.Parse(source)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if os.Args[1] == "outline" {
		if err := enc.Encode(doc.Symbols()); err != nil {
			fail(err)
		}
		return
	}
	diagnostics := doc.Diagnostics()
	if err := enc.Encode(diagnostics); err != nil {
		fail(err)
	}
	if len(diagnostics) > 0 {
		os.Exit(1)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
