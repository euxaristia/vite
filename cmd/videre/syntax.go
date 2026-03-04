package main

import (
	"bytes"
	"path/filepath"
	"strings"
)

var syntaxes = []syntax{
	{filetype: "c", exts: []string{".c", ".h"}, kws: kwList([]string{"if", "else", "for", "while", "switch", "case", "return", "struct|", "int|", "char|", "void|"}), lineCmt: "//"},
	{filetype: "go", exts: []string{".go"}, kws: kwList([]string{"package", "import", "func", "type", "struct", "interface", "if", "else", "for", "range", "return", "map|", "string|", "int|", "bool|", "error|"}), lineCmt: "//"},
	{filetype: "rust", exts: []string{".rs"}, kws: kwList([]string{"fn", "let", "mut", "if", "else", "match", "impl", "struct", "enum", "use", "pub", "String|", "Vec|"}), lineCmt: "//"},
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

func updateSyntax(r *row, force bool) {
	if !force && !r.needsHighlight {
		return
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
		return
	}
	lineCmt := E.syntax.lineCmt
	lineCmtB := []byte(lineCmt)
	lineCmtLen := len(lineCmtB)
	lineCmtFirst := byte(0)
	if lineCmtLen > 0 {
		lineCmtFirst = lineCmtB[0]
	}
	for i := 0; i < n; {
		if lineCmtLen > 0 && r.s[i] == lineCmtFirst {
			isLineComment := false
			if lineCmtLen == 1 {
				isLineComment = true
			} else if lineCmtLen == 2 {
				isLineComment = i+1 < n && r.s[i+1] == lineCmtB[1]
			} else if i+lineCmtLen <= n {
				isLineComment = bytes.Equal(r.s[i:i+lineCmtLen], lineCmtB)
			}
			if isLineComment {
				for j := i; j < n; j++ {
					r.hl[j] = hlComment
				}
				break
			}
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
}

func updateAllSyntax(force bool) {
	for i := range E.rows {
		updateSyntax(&E.rows[i], force)
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
