package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/M31-Labs/bend-intel/intel"
)

func main() {
	commands := map[string]bool{"check": true, "outline": true, "status": true, "calls": true, "parallel": true, "patterns": true, "hvm": true}
	if len(os.Args) != 3 || !commands[os.Args[1]] {
		fmt.Fprintln(os.Stderr, "usage: bend-intel <check|outline|status|calls|parallel|patterns|hvm> <file.bend>")
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
	if os.Args[1] == "status" {
		if err := enc.Encode(doc.Health()); err != nil {
			fail(err)
		}
		return
	}
	if os.Args[1] == "calls" {
		if err := enc.Encode(doc.CallSites()); err != nil {
			fail(err)
		}
		return
	}
	if os.Args[1] == "parallel" {
		if err := enc.Encode(doc.ParallelStructure()); err != nil {
			fail(err)
		}
		return
	}
	if os.Args[1] == "patterns" {
		if err := enc.Encode(doc.PatternCoverage()); err != nil {
			fail(err)
		}
		return
	}
	if os.Args[1] == "hvm" {
		if _, err := fmt.Fprintln(os.Stdout, doc.HVMView()); err != nil {
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
