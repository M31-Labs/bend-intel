//go:build cgo && parity

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/M31-Labs/bend-intel/internal/cparity"
)

type report struct {
	Path    string `json:"path"`
	Class   string `json:"class,omitempty"`
	Equal   bool   `json:"equal"`
	CError  bool   `json:"cError"`
	GoError bool   `json:"goError"`
	Error   string `json:"error,omitempty"`
	CTree   string `json:"cTree,omitempty"`
	GoTree  string `json:"goTree,omitempty"`
}

func main() {
	root := flag.String("path", ".", "Bend file or directory to compare")
	class := flag.String("class", "", "fixture class to include in each result")
	flag.Parse()
	paths, err := collect(*root)
	if err != nil {
		fail(err)
	}
	results := make([]report, 0, len(paths))
	for _, path := range paths {
		source, readErr := os.ReadFile(path)
		item := report{Path: path, Class: *class}
		if readErr != nil {
			item.Error = readErr.Error()
			results = append(results, item)
			continue
		}
		parity, compareErr := cparity.Compare(source)
		if compareErr != nil {
			item.Error = compareErr.Error()
		} else {
			item.Equal, item.CError, item.GoError, item.CTree, item.GoTree = parity.Equal, parity.CError, parity.GoError, parity.CTree, parity.GoTree
		}
		results = append(results, item)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fail(err)
	}
	for _, item := range results {
		if !item.Equal || item.Error != "" {
			os.Exit(1)
		}
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

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
