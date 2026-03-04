package main

import (
	"bytes"
	"path/filepath"
	"strings"
)

type syntax struct {
	filetype string
	exts     []string
	kws      []keyword
	lineCmt  string
	blkCmtS  string
	blkCmtE  string
}

var syntaxes = []syntax{
	{filetype: "c", exts: []string{".c", ".h"}, kws: kwList([]string{"if", "else", "for", "while", "switch", "case", "return", "struct|", "int|", "char|", "void|"}), lineCmt: "//", blkCmtS: "/*", blkCmtE: "*/"},
	{filetype: "go", exts: []string{".go"}, kws: kwList([]string{"package", "import", "func", "type", "struct", "interface", "if", "else", "for", "range", "return", "map|", "string|", "int|", "bool|", "error|"}), lineCmt: "//", blkCmtS: "/*", blkCmtE: "*/"},
	{filetype: "rust", exts: []string{".rs"}, kws: kwList([]string{"fn", "let", "mut", "if", "else", "match", "impl", "struct", "enum", "use", "pub", "String|", "Vec|"}), lineCmt: "//", blkCmtS: "/*", blkCmtE: "*/"},
	{filetype: "python", exts: []string{".py"}, kws: kwList([]string{"def", "class", "if", "elif", "else", "for", "while", "return", "import", "from", "None", "True", "False"}), lineCmt: "#"},
}

func kwList(src []string) []keyword {
	out := make([]keyword, 0, len(src))
	for _, kw := range src {
		kind := hlKeyword1
		if strings.HasSuffix(kw, "|") {
			kw = strings.TrimSuffix(kw, "|")
			kind = hlKeyword2
		}
		out = append(out, keyword{lit: []byte(kw), kind: kind})
	}
	return out
}

func keywordKind(kws []keyword, token []byte) (uint8, bool) {
	for _, kw := range kws {
		if len(kw.lit) == len(token) && bytes.Equal(kw.lit, token) {
			return kw.kind, true
		}
	}
	return 0, false
}

func updateSyntax(r *row, force bool) bool {
	prevHlState := 0
	if r.idx > 0 {
		prevHlState = E.rows[r.idx-1].hlState
	}
	if !force && !r.needsHighlight {
		// Even if we don't re-parse the line, we need to return if the state changed
		// to allow propagation.
		return false
	}
	r.needsHighlight = false
	n := len(r.s)
	if cap(r.hl) < len(r.s) {
		r.hl = make([]uint8, n)
	} else {
		r.hl = r.hl[:n]
		for i := range r.hl {
			r.hl[i] = hlNormal
		}
	}
	if E.syntax == nil {
		r.hlState = 0
		return false
	}

	lineCmt := E.syntax.lineCmt
	lineCmtB := []byte(lineCmt)
	lineCmtLen := len(lineCmtB)
	
	blkS := []byte(E.syntax.blkCmtS)
	blkE := []byte(E.syntax.blkCmtE)
	inBlk := prevHlState != 0

	for i := 0; i < n; {
		if inBlk {
			r.hl[i] = hlComment
			if len(blkE) > 0 && i+len(blkE) <= n && bytes.Equal(r.s[i:i+len(blkE)], blkE) {
				for j := 0; j < len(blkE); j++ {
					r.hl[i+j] = hlComment
				}
				i += len(blkE)
				inBlk = false
				continue
			}
			i++
			continue
		}

		if len(blkS) > 0 && i+len(blkS) <= n && bytes.Equal(r.s[i:i+len(blkS)], blkS) {
			inBlk = true
			for j := 0; j < len(blkS); j++ {
				r.hl[i+j] = hlComment
			}
			i += len(blkS)
			continue
		}

		if lineCmtLen > 0 && i+lineCmtLen <= n && bytes.Equal(r.s[i:i+lineCmtLen], lineCmtB) {
			for j := i; j < n; j++ {
				r.hl[j] = hlComment
			}
			break
		}
		if r.s[i] == '"' || r.s[i] == '\'' {
			q := r.s[i]
			r.hl[i] = hlString
			i++
			for i < n {
				r.hl[i] = hlString
				if r.s[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if r.s[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		if isDigitByte(r.s[i]) {
			j := i
			for j < n && (isDigitByte(r.s[j]) || r.s[j] == '.') {
				j++
			}
			for k := i; k < j; k++ {
				r.hl[k] = hlNumber
			}
			i = j
			continue
		}
		if isAlphaByte(r.s[i]) || r.s[i] == '_' {
			j := i
			for j < n && isWordByte(r.s[j]) {
				j++
			}
			if t, ok := keywordKind(E.syntax.kws, r.s[i:j]); ok {
				for k := i; k < j; k++ {
					r.hl[k] = t
				}
			}
			i = j
			continue
		}
		i++
	}
	
	newHlState := 0
	if inBlk {
		newHlState = 1
	}
	
	stateChanged := r.hlState != newHlState
	r.hlState = newHlState

	if len(E.searchBytes) > 0 {
		q := E.searchBytes
		for off := 0; ; {
			m := bytes.Index(r.s[off:], q)
			if m < 0 {
				break
			}
			m += off
			for i := m; i < m+len(q) && i < len(r.hl); i++ {
				if r.idx == E.cy && m <= E.cx && E.cx < m+len(q) {
					r.hl[i] = hlMatchCursor
				} else {
					r.hl[i] = hlMatch
				}
			}
			off = m + 1
		}
	}
	return stateChanged
}

func updateAllSyntax(force bool) {
	for i := range E.rows {
		changed := updateSyntax(E.rows[i], force)
		if changed && i+1 < len(E.rows) {
			E.rows[i+1].needsHighlight = true
		}
	}
}

func selectSyntax() {
	E.syntax = nil
	if E.filename == "" {
		updateAllSyntax(true)
		return
	}
	ext := strings.ToLower(filepath.Ext(E.filename))
	for i := range syntaxes {
		for _, e := range syntaxes[i].exts {
			if e == ext {
				E.syntax = &syntaxes[i]
				updateAllSyntax(true)
				return
			}
		}
	}
	updateAllSyntax(true)
}

func setSearchPattern(p string) {
	E.searchPattern = p
	if p == "" {
		E.searchBytes = nil
		return
	}
	E.searchBytes = append(E.searchBytes[:0], p...)
}
