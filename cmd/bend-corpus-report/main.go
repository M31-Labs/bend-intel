package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/M31-Labs/bend-intel/bendlang"
	"github.com/M31-Labs/bend-intel/intel"
	gotreesitter "github.com/odvcencio/gotreesitter"
)

type fileReport struct {
	Path        string `json:"path"`
	RawErrors   int    `json:"rawErrors"`
	IntelErrors int    `json:"intelErrors"`
	Recovered   bool   `json:"recovered"`
	SymbolCount int    `json:"symbolCount"`
	ParseError  string `json:"parseError,omitempty"`
}

type corpusReport struct {
	Root        string       `json:"root"`
	Files       int          `json:"files"`
	RawClean    int          `json:"rawClean"`
	IntelClean  int          `json:"intelClean"`
	Recovered   int          `json:"recovered"`
	WithSymbols int          `json:"withSymbols"`
	Diagnostics int          `json:"diagnostics"`
	Entries     []fileReport `json:"entries"`
}

func main() {
	root := flag.String("path", ".", "Bend file or directory to inspect")
	flag.Parse()
	paths, err := collect(*root)
	if err != nil {
		fail(err)
	}
	report := corpusReport{Root: *root, Entries: make([]fileReport, 0, len(paths))}
	for _, path := range paths {
		entry := fileReport{Path: path}
		source, err := os.ReadFile(path)
		if err != nil {
			entry.ParseError = err.Error()
			report.Entries = append(report.Entries, entry)
			continue
		}
		rawParser, err := bendlang.NewParser()
		if err != nil {
			entry.ParseError = err.Error()
			report.Entries = append(report.Entries, entry)
			continue
		}
		rawTree, err := rawParser.Parse(source)
		if err != nil {
			entry.ParseError = err.Error()
		} else {
			entry.RawErrors = errorCount(rawTree.RootNode())
		}
		doc, err := intel.Parse(source)
		if err != nil {
			if entry.ParseError == "" {
				entry.ParseError = err.Error()
			}
		} else {
			for _, diagnostic := range doc.Diagnostics() {
				if diagnostic.Severity <= 2 {
					entry.IntelErrors++
				}
			}
			entry.Recovered = doc.Recovered()
			entry.SymbolCount = len(doc.Symbols())
		}
		report.Entries = append(report.Entries, entry)
	}
	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].Path < report.Entries[j].Path })
	for _, entry := range report.Entries {
		report.Files++
		if entry.RawErrors == 0 && entry.ParseError == "" {
			report.RawClean++
		}
		if entry.IntelErrors == 0 && entry.ParseError == "" {
			report.IntelClean++
		}
		if entry.Recovered {
			report.Recovered++
		}
		if entry.SymbolCount > 0 {
			report.WithSymbols++
		}
		report.Diagnostics += entry.IntelErrors
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

func collect(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	var paths []string
	err = filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".bend" {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func errorCount(node *gotreesitter.Node) int {
	if node == nil {
		return 0
	}
	count := 0
	if node.IsError() {
		count++
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		count += errorCount(node.NamedChild(i))
	}
	return count
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
