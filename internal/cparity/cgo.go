//go:build cgo && parity

// Package cparity contains the optional C-runtime witness used to validate the
// pure-Go Bend grammar. It is intentionally isolated from bendlang so normal
// consumers never acquire a CGo dependency.
package cparity

/*
#cgo CFLAGS: -I${SRCDIR}/c
#include "parser.c"
#include "scanner.c"
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/M31-Labs/bend-intel/bendlang"
	gotreesitter "github.com/odvcencio/gotreesitter"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Parse returns the C Tree-sitter named S-expression for source.
func Parse(source []byte) (string, error) {
	parser, tree, err := parseCTree(source)
	if err != nil {
		return "", err
	}
	defer parser.Close()
	defer tree.Close()
	return tree.RootNode().ToSexp(), nil
}

func parseCTree(source []byte) (*sitter.Parser, *sitter.Tree, error) {
	parser := sitter.NewParser()
	if err := parser.SetLanguage(sitter.NewLanguage(unsafe.Pointer(C.tree_sitter_bend()))); err != nil {
		parser.Close()
		return nil, nil, fmt.Errorf("set C Bend language: %w", err)
	}
	tree := parser.Parse(source, nil)
	if tree == nil {
		parser.Close()
		return nil, nil, fmt.Errorf("C parser returned nil tree")
	}
	return parser, tree, nil
}

type Result struct {
	Equal   bool
	CError  bool
	GoError bool
	CTree   string
	GoTree  string
}

// Compare parses the same bytes through both runtimes and compares their
// named S-expressions. Anonymous punctuation and error/missing nodes are
// intentionally retained: a mismatch must be actionable for grammar authors.
func Compare(source []byte) (Result, error) {
	cParser, cTree, err := parseCTree(source)
	if err != nil {
		return Result{}, err
	}
	defer cParser.Close()
	defer cTree.Close()
	lang, err := bendlang.Language()
	if err != nil {
		return Result{}, err
	}
	parser, err := bendlang.NewParser()
	if err != nil {
		return Result{}, err
	}
	goTree, err := parser.Parse(source)
	if err != nil {
		return Result{}, err
	}
	goSExpr := goShape(goTree.RootNode(), lang)
	cSExpr := cTree.RootNode().ToSexp()
	return Result{Equal: cSExpr == goSExpr && metadataEqual(cTree.RootNode(), goTree.RootNode(), lang), CError: strings.Contains(cSExpr, "(ERROR"), GoError: strings.Contains(goSExpr, "(ERROR"), CTree: cSExpr, GoTree: goSExpr}, nil
}

func goShape(node *gotreesitter.Node, lang *gotreesitter.Language) string {
	var out strings.Builder
	out.WriteByte('(')
	out.WriteString(node.Type(lang))
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || !child.IsNamed() {
			continue
		}
		out.WriteByte(' ')
		if field := node.FieldNameForChild(i, lang); field != "" {
			out.WriteString(field)
			out.WriteString(": ")
		}
		out.WriteString(goShape(child, lang))
	}
	out.WriteByte(')')
	return out.String()
}

func metadataEqual(cNode *sitter.Node, goNode *gotreesitter.Node, lang *gotreesitter.Language) bool {
	if cNode.Kind() != goNode.Type(lang) || cNode.IsNamed() != goNode.IsNamed() || cNode.IsMissing() != goNode.IsMissing() || cNode.IsExtra() != goNode.IsExtra() || cNode.StartByte() != uint(goNode.StartByte()) || cNode.EndByte() != uint(goNode.EndByte()) {
		return false
	}
	if cNode.ChildCount() != uint(goNode.ChildCount()) {
		return false
	}
	for i := uint(0); i < cNode.ChildCount(); i++ {
		cChild, goChild := cNode.Child(i), goNode.Child(int(i))
		if cChild == nil || goChild == nil {
			if cChild != nil || goChild != nil {
				return false
			}
			continue
		}
		if cNode.FieldNameForChild(uint32(i)) != goNode.FieldNameForChild(int(i), lang) || !metadataEqual(cChild, goChild, lang) {
			return false
		}
	}
	return true
}
