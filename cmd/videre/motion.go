package main

func moveCursor(key int) {
	var r *row
	if E.cy >= 0 && E.cy < len(E.rows) {
		r = &E.rows[E.cy]
	}
	switch key {
	case arrowLeft:
		if E.cx > 0 && r != nil {
			E.cx = utf8PrevBoundary(r.s, E.cx)
		} else if E.cy > 0 {
			E.cy--
			E.cx = len(E.rows[E.cy].s)
			if E.mode != modeInsert && E.cx > 0 {
				E.cx = utf8PrevBoundary(E.rows[E.cy].s, E.cx)
			}
		}
	case arrowRight:
		if r != nil && E.cx < len(r.s) {
			E.cx = utf8NextBoundary(r.s, E.cx)
		} else if r != nil && E.mode == modeInsert && E.cy < len(E.rows)-1 {
			E.cy++
			E.cx = 0
		}
	case arrowUp:
		if E.cy > 0 {
			E.cy--
		}
	case arrowDown:
		if E.cy < len(E.rows)-1 {
			E.cy++
		}
	}
	if E.cy < 0 {
		E.cy = 0
	}
	if len(E.rows) == 0 {
		E.cx = 0
		return
	}
	if E.cy >= len(E.rows) {
		E.cy = len(E.rows) - 1
	}
	limit := len(E.rows[E.cy].s)
	if E.mode != modeInsert && limit > 0 {
		limit = utf8PrevBoundary(E.rows[E.cy].s, limit)
	}
	if key == arrowUp || key == arrowDown {
		if E.preferred > limit {
			E.cx = limit
		} else {
			E.cx = E.preferred
		}
	} else {
		if E.cx > limit {
			E.cx = limit
		}
		E.preferred = E.cx
	}
}

func moveLeftNoWrap() {
	if E.cy < 0 || E.cy >= len(E.rows) {
		E.cx = 0
		E.preferred = 0
		return
	}
	if E.cx > 0 {
		E.cx = utf8PrevBoundary(E.rows[E.cy].s, E.cx)
	}
	E.preferred = E.cx
}

func moveRightNoWrap() {
	if E.cy < 0 || E.cy >= len(E.rows) {
		E.cx = 0
		E.preferred = 0
		return
	}
	line := E.rows[E.cy].s
	limit := len(line)
	if E.mode != modeInsert && limit > 0 {
		limit = utf8PrevBoundary(line, limit)
	}
	if E.cx < limit {
		E.cx = utf8NextBoundary(line, E.cx)
		if E.cx > limit {
			E.cx = limit
		}
	}
	E.preferred = E.cx
}

func moveWordForward(big bool) {
	if len(E.rows) == 0 {
		return
	}
	r, c := E.cy, E.cx
	for r < len(E.rows) {
		line := E.rows[r].s
		if c < len(line) {
			if big {
				for c < len(line) && line[c] != ' ' && line[c] != '\t' {
					c++
				}
			} else {
				if isWordChar(line[c]) {
					for c < len(line) && isWordChar(line[c]) {
						c++
					}
				} else {
					for c < len(line) && !isWordChar(line[c]) && line[c] != ' ' && line[c] != '\t' {
						c++
					}
				}
			}
		}
		for c < len(line) && (line[c] == ' ' || line[c] == '\t') {
			c++
		}
		if c < len(line) {
			E.cy, E.cx, E.preferred = r, c, c
			return
		}
		r++
		c = 0
	}
	E.cy = len(E.rows) - 1
	E.cx = len(E.rows[E.cy].s)
	E.preferred = E.cx
}

func moveWordBackward(big bool) {
	if len(E.rows) == 0 {
		return
	}
	r, c := E.cy, E.cx-1
	for r >= 0 {
		line := E.rows[r].s
		for c >= 0 && (line[c] == ' ' || line[c] == '\t') {
			c--
		}
		if c >= 0 {
			if big {
				for c >= 0 && line[c] != ' ' && line[c] != '\t' {
					c--
				}
			} else {
				if isWordChar(line[c]) {
					for c >= 0 && isWordChar(line[c]) {
						c--
					}
				} else {
					for c >= 0 && !isWordChar(line[c]) && line[c] != ' ' && line[c] != '\t' {
						c--
					}
				}
			}
			E.cy, E.cx, E.preferred = r, c+1, c+1
			return
		}
		r--
		if r >= 0 {
			c = len(E.rows[r].s) - 1
		}
	}
	E.cy, E.cx, E.preferred = 0, 0, 0
}

func moveLineStart() { E.cx, E.preferred = 0, 0 }

func moveFirstNonWhitespace() {
	if E.cy < 0 || E.cy >= len(E.rows) {
		return
	}
	col := 0
	line := E.rows[E.cy].s
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	E.cx, E.preferred = col, col
}

func moveLineEnd() {
	if E.cy >= 0 && E.cy < len(E.rows) {
		E.cx = len(E.rows[E.cy].s)
		E.preferred = E.cx
	}
}
func moveFileStart() { E.cy, E.cx, E.preferred = 0, 0, 0 }
func moveFileEnd() {
	if len(E.rows) == 0 {
		return
	}
	E.cy = len(E.rows) - 1
	E.cx = len(E.rows[E.cy].s)
	E.preferred = E.cx
}

func moveWordEnd(big bool) {
	if len(E.rows) == 0 {
		return
	}
	r, c := E.cy, E.cx+1
	for r < len(E.rows) {
		line := E.rows[r].s
		for c < len(line) && (line[c] == ' ' || line[c] == '\t') {
			c++
		}
		if c < len(line) {
			if big {
				for c < len(line)-1 && line[c+1] != ' ' && line[c+1] != '\t' {
					c++
				}
			} else {
				if isWordChar(line[c]) {
					for c < len(line)-1 && isWordChar(line[c+1]) {
						c++
					}
				} else {
					for c < len(line)-1 && !isWordChar(line[c+1]) && line[c+1] != ' ' && line[c+1] != '\t' {
						c++
					}
				}
			}
			E.cy, E.cx, E.preferred = r, c, c
			return
		}
		r++
		c = 0
	}
}

func matchBracket() {
	if E.cy < 0 || E.cy >= len(E.rows) {
		return
	}

	line := E.rows[E.cy].s
	found := false
	for x := E.cx; x < len(line); x++ {
		ch := line[x]
		if ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '{' || ch == '}' {
			E.cx = x
			found = true
			break
		}
	}

	if !found {
		return
	}

	cur := E.rows[E.cy].s[E.cx]
	var target byte
	dir := 0
	switch cur {
	case '(':
		target, dir = ')', 1
	case ')':
		target, dir = '(', -1
	case '[':
		target, dir = ']', 1
	case ']':
		target, dir = '[', -1
	case '{':
		target, dir = '}', 1
	case '}':
		target, dir = '{', -1
	default:
		return
	}
	depth := 1
	ry, rx := E.cy, E.cx+dir
	for ry >= 0 && ry < len(E.rows) {
		rowBytes := E.rows[ry].s
		for rx >= 0 && rx < len(rowBytes) {
			ch := rowBytes[rx]
			if ch == cur {
				depth++
			} else if ch == target {
				depth--
				if depth == 0 {
					E.cy, E.cx, E.preferred = ry, rx, rx
					return
				}
			}
			rx += dir
		}
		ry += dir
		if dir > 0 {
			rx = 0
		} else if ry >= 0 {
			rx = len(E.rows[ry].s) - 1
		}
	}
}

func isBlankRow(idx int) bool {
	if idx < 0 || idx >= len(E.rows) {
		return true
	}
	line := E.rows[idx].s
	if len(line) == 0 {
		return true
	}
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			return false
		}
	}
	return true
}

func movePreviousParagraph() {
	if len(E.rows) == 0 {
		return
	}
	row := E.cy
	if !isBlankRow(row) {
		for row > 0 && !isBlankRow(row-1) {
			row--
		}
	}
	for row > 0 && isBlankRow(row-1) {
		row--
	}
	for row > 0 && !isBlankRow(row-1) {
		row--
	}
	E.cy, E.cx, E.preferred = row, 0, 0
}

func moveNextParagraph() {
	if len(E.rows) == 0 {
		return
	}
	row := E.cy
	if !isBlankRow(row) {
		for row < len(E.rows)-1 && !isBlankRow(row+1) {
			row++
		}
	}
	for row < len(E.rows)-1 && isBlankRow(row+1) {
		row++
	}
	if row < len(E.rows)-1 {
		row++
	}
	E.cy, E.cx, E.preferred = row, 0, 0
}

func moveToLine(n int) {
	if len(E.rows) == 0 {
		E.cy, E.cx, E.preferred = 0, 0, 0
		return
	}
	if n < 1 {
		n = 1
	}
	E.cy = min(n-1, len(E.rows)-1)
	if E.cx > len(E.rows[E.cy].s) {
		E.cx = len(E.rows[E.cy].s)
	}
	E.preferred = E.cx
}
