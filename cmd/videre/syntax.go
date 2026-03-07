package main

import (
	"context"
	"path/filepath"
	"regexp"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/rust"
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

	ext := filepath.Ext(E.filename)
	var lang *sitter.Language
	switch ext {
	case ".go":
		lang = golang.GetLanguage()
	case ".rs":
		lang = rust.GetLanguage()
	}

	if lang != nil {
		parser := sitter.NewParser()
		parser.SetLanguage(lang)
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
		case "comment", "line_comment", "block_comment":
			hl = hlComment
		case "string_literal", "raw_string_literal", "char_literal":
			hl = hlString
		case "int_literal", "float_literal", "imaginary_literal", "integer_literal":
			hl = hlNumber
		case "func", "package", "import", "type", "struct", "interface", "return", "if", "else", "for", "range", "go", "defer", "map", "chan", "var", "const", "fn", "let", "mut", "match", "impl", "enum", "use", "pub", "mod", "trait", "where", "async", "await":
			hl = hlKeyword1
		case "string", "int", "bool", "error", "byte", "rune", "uint", "uintptr", "float32", "float64", "complex64", "complex128", "String", "Vec", "Option", "Result", "u8", "u16", "u32", "u64", "u128", "i8", "i16", "i32", "i64", "i128", "f32", "f64", "usize", "isize":
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
