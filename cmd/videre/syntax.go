package main

import (
	"context"
	"path/filepath"
	"regexp"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

func updateSyntax(r *row, force bool) bool {
	if !force && !r.needsHighlight {
		return false
	}
	r.needsHighlight = false
	n := len(r.s)
	if cap(r.hl) < n {
		r.hl = make([]uint8, n)
	} else {
		r.hl = r.hl[:n]
		for i := range r.hl {
			r.hl[i] = hlNormal
		}
	}

	if filepath.Ext(E.filename) == ".go" {
		parser := sitter.NewParser()
		parser.SetLanguage(golang.GetLanguage())
		tree, _ := parser.ParseCtx(context.Background(), nil, r.s)
		if tree != nil {
			node := tree.RootNode()
			applyTreeSitterHighlight(r, node)
		}
	}

	if E.searchRegexp != nil {
		matches := E.searchRegexp.FindAllIndex(r.s, -1)
		for _, m := range matches {
			start, end := m[0], m[1]
			for i := start; i < end && i < len(r.hl); i++ {
				if r.idx == E.cy && start <= E.cx && E.cx < end {
					r.hl[i] = hlMatchCursor
				} else {
					r.hl[i] = hlMatch
				}
			}
		}
	}
	return false
}

func applyTreeSitterHighlight(r *row, n *sitter.Node) {
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		kind := child.Type()
		start := int(child.StartByte())
		end := int(child.EndByte())
		
		var hl uint8 = hlNormal
		switch kind {
		case "comment":
			hl = hlComment
		case "string_literal", "raw_string_literal":
			hl = hlString
		case "int_literal", "float_literal", "imaginary_literal":
			hl = hlNumber
		case "func", "package", "import", "type", "struct", "interface", "return", "if", "else", "for", "range", "go", "defer", "map", "chan", "var", "const":
			hl = hlKeyword1
		case "string", "int", "bool", "error", "byte", "rune", "uint", "uintptr", "float32", "float64", "complex64", "complex128":
			hl = hlKeyword2
		}
		
		if hl != hlNormal {
			for j := start; j < end && j < len(r.hl); j++ {
				r.hl[j] = hl
			}
		}
		applyTreeSitterHighlight(r, child)
	}
}

func updateAllSyntax(force bool) {
	for i := range E.rows {
		updateSyntax(E.rows[i], force)
	}
}

func selectSyntax() {}
func setSearchPattern(p string) {
	E.searchPattern = p
	if p == "" {
		E.searchRegexp = nil
		return
	}
	re, err := regexp.Compile("(?i)" + p)
	if err == nil {
		E.searchRegexp = re
	}
}
