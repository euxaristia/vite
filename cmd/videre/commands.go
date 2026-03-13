package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func startRecording(initial int) {
	E.recordingChange = true
	E.currentChange = []int{initial}
}

func stopRecording() {
	if E.recordingChange {
		E.lastChange = make([]int, len(E.currentChange))
		copy(E.lastChange, E.currentChange)
		E.recordingChange = false
	}
}

func changeCase(toUpper bool) {
	if E.mode != modeVisual && E.mode != modeVisualLine {
		return
	}
	sy, sx := E.selSY, E.selSX
	ey, ex := E.cy, E.cx
	if sy > ey || (sy == ey && sx > ex) {
		sy, ey = ey, sy
		sx, ex = ex, sx
	}
	saveUndo()
	for y := sy; y <= ey && y < len(E.rows); y++ {
		E.rows[y] = duplicateRow(E.rows[y])
		line := E.rows[y].s
		start := 0
		end := len(line)
		if y == sy {
			start = max(0, sx)
		}
		if y == ey {
			end = min(len(line), ex+1)
		}
		for i := start; i < end; i++ {
			if toUpper {
				line[i] = byte(unicode.ToUpper(rune(line[i])))
			} else {
				line[i] = byte(unicode.ToLower(rune(line[i])))
			}
		}
		updateSyntax(E.rows[y], false)
	}
	E.dirty = true
	E.mode = modeNormal
	E.selSX, E.selSY = -1, -1
}

func indentSelection(indent bool) {
	if E.mode != modeVisual && E.mode != modeVisualLine {
		return
	}
	sy, ey := E.selSY, E.cy
	if sy > ey {
		sy, ey = ey, sy
	}
	saveUndo()
	for y := sy; y <= ey && y < len(E.rows); y++ {
		E.rows[y] = duplicateRow(E.rows[y])
		if indent {
			E.rows[y].s = append([]byte("    "), E.rows[y].s...)
		} else {
			trim := 0
			for trim < 4 && trim < len(E.rows[y].s) && E.rows[y].s[trim] == ' ' {
				trim++
			}
			if trim > 0 {
				E.rows[y].s = E.rows[y].s[trim:]
			}
		}
		updateSyntax(E.rows[y], false)
	}
	E.dirty = true
	E.mode = modeNormal
	E.selSX, E.selSY = -1, -1
}

func findCharInternal(c byte, direction int, till bool, record bool) bool {
	if record {
		E.lastSearchChar = c
		E.lastSearchDir = direction
		E.lastSearchTill = till
	}
	if len(E.rows) == 0 {
		return false
	}
	if E.cy < 0 || E.cy >= len(E.rows) {
		return false
	}
	line := E.rows[E.cy].s
	if len(line) == 0 {
		return false
	}
	start := E.cx
	if direction > 0 {
		start++
	} else {
		start--
	}
	if direction > 0 {
		if start < 0 {
			start = 0
		}
		for x := start; x < len(line); x++ {
			if line[x] == c {
				if till {
					x--
					if x < 0 {
						x = 0
					}
				}
				E.cx, E.preferred = x, x
				return true
			}
		}
	} else {
		if start >= len(line) {
			start = len(line) - 1
		}
		for x := start; x >= 0; x-- {
			if line[x] == c {
				if till {
					x++
					if x >= len(line) {
						x = len(line) - 1
					}
				}
				E.cx, E.preferred = x, x
				return true
			}
		}
	}
	return false
}

func findChar(c byte, direction int, till bool) bool {
	return findCharInternal(c, direction, till, true)
}

func repeatCharSearch(reverse bool) {
	if E.lastSearchChar == 0 || E.lastSearchDir == 0 {
		return
	}
	dir := E.lastSearchDir
	if reverse {
		dir = -dir
	}
	_ = findCharInternal(E.lastSearchChar, dir, E.lastSearchTill, false)
}

func selectAll() {
	E.mode = modeVisual
	E.selSY, E.selSX = 0, 0
	if len(E.rows) == 0 {
		E.cy, E.cx = 0, 0
		E.preferred = 0
		return
	}
	E.cy = len(E.rows) - 1
	E.cx = len(E.rows[E.cy].s)
	E.preferred = E.cx
}

func incrementNumber(delta int) {
	if E.cy < 0 || E.cy >= len(E.rows) {
		return
	}
	line := E.rows[E.cy].s
	i := E.cx
	for i < len(line) && !(line[i] >= '0' && line[i] <= '9') {
		if line[i] == '-' && i+1 < len(line) && line[i+1] >= '0' && line[i+1] <= '9' {
			break
		}
		i++
	}
	if i >= len(line) {
		return
	}
	j := i
	if line[j] == '-' {
		j++
	}
	for j < len(line) && line[j] >= '0' && line[j] <= '9' {
		j++
	}
	n, err := strconv.Atoi(string(line[i:j]))
	if err != nil {
		return
	}
	n += delta
	saveUndo()
	repl := []byte(strconv.Itoa(n))
	newLine := make([]byte, 0, len(line)-(j-i)+len(repl))
	newLine = append(newLine, line[:i]...)
	newLine = append(newLine, repl...)
	newLine = append(newLine, line[j:]...)
	E.rows[E.cy] = duplicateRow(E.rows[E.cy])
	E.rows[E.cy].s = newLine
	E.cx = i + len(repl) - 1
	if E.cx < 0 {
		E.cx = 0
	}
	E.preferred = E.cx
	E.rows[E.cy].needsHighlight = true
	updateSyntax(E.rows[E.cy], false)
	E.dirty = true
}

func setClipboard(text []byte) {
	if len(text) == 0 {
		return
	}
	if E.InTest {
		return
	}
	encoded := base64.StdEncoding.EncodeToString(text)
	fmt.Printf("\x1b]52;c;%s\x07", encoded)
	_ = os.Stdout.Sync()

	if cmd := exec.Command("wl-copy"); cmd != nil {
		cmd.Stdin = bytes.NewReader(text)
		_ = cmd.Run()
	}
	if cmd := exec.Command("xclip", "-selection", "clipboard"); cmd != nil {
		cmd.Stdin = bytes.NewReader(text)
		_ = cmd.Run()
	}
}

func getClipboard() []byte {
	if E.InTest {
		return nil
	}
	if out, err := exec.Command("wl-paste", "-n").Output(); err == nil && len(out) > 0 {
		return out
	}
	if out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output(); err == nil && len(out) > 0 {
		return out
	}
	return nil
}

func yoink(sx, sy, ex, ey int, isLine bool) {
	if sy > ey || (sy == ey && sx > ex) {
		sx, ex = ex, sx
		sy, ey = ey, sy
	}
	var b bytes.Buffer
	if isLine {
		for i := sy; i <= ey && i < len(E.rows); i++ {
			b.Write(E.rows[i].s)
			b.WriteByte('\n')
		}
	} else if sy == ey && sy < len(E.rows) {
		r := E.rows[sy].s
		if sx < 0 {
			sx = 0
		}
		if ex >= len(r) {
			ex = len(r) - 1
		}
		if sx <= ex {
			b.Write(r[sx : ex+1])
		}
	} else {
		for i := sy; i <= ey && i < len(E.rows); i++ {
			r := E.rows[i].s
			if i == sy {
				if sx < len(r) {
					b.Write(r[sx:])
				}
				b.WriteByte('\n')
			} else if i == ey {
				if ex >= len(r) {
					ex = len(r) - 1
				}
				if ex >= 0 {
					b.Write(r[:ex+1])
				}
			} else {
				b.Write(r)
				b.WriteByte('\n')
			}
		}
	}

	regName := E.SelectedRegister
	if regName == 0 {
		regName = '"'
	}

	if regName == '_' {
		return
	}

	E.registers[regName] = reg{s: b.Bytes(), isLine: isLine}
	if regName == '"' {
		setClipboard(E.registers['"'].s)
	}
}

func deleteRange(sx, sy, ex, ey int) {
	if sy > ey || (sy == ey && sx > ex) {
		sx, ex = ex, sx
		sy, ey = ey, sy
	}
	saveUndo()
	if sy == ey {
		E.rows[sy] = duplicateRow(E.rows[sy])
		r := E.rows[sy]
		if sx < 0 {
			sx = 0
		}
		if ex >= len(r.s) {
			ex = len(r.s) - 1
		}
		if sx <= ex {
			r.s = append(r.s[:sx], r.s[ex+1:]...)
			updateSyntax(r, false)
		}
	} else {
		E.rows[sy] = duplicateRow(E.rows[sy])
		first := append([]byte(nil), E.rows[sy].s[:sx]...)
		if ey < len(E.rows) {
			last := E.rows[ey].s
			if ex+1 < len(last) {
				first = append(first, last[ex+1:]...)
			}
		}
		E.rows[sy].s = first
		updateSyntax(E.rows[sy], false)
		for i := 0; i < ey-sy; i++ {
			delRow(sy + 1)
		}
	}
	E.cy, E.cx = sy, sx
	if E.cy >= len(E.rows) {
		E.cy = len(E.rows) - 1
	}
	if E.cy < 0 {
		E.cy = 0
	}
}

func paste() {
	if clip := getClipboard(); len(clip) > 0 {
		E.registers['"'] = reg{s: append([]byte(nil), clip...), isLine: bytes.Contains(clip, []byte{'\n'})}
	}
	r := E.registers['"']
	if len(r.s) == 0 {
		return
	}
	saveUndo()
	if r.isLine {
		lines := strings.Split(string(r.s), "\n")
		at := E.cy + 1
		for _, ln := range lines {
			if ln == "" {
				continue
			}
			insertRow(at, []byte(ln))
			at++
		}
		return
	}
	for _, c := range r.s {
		if c == '\n' {
			insertNewline()
		} else {
			insertChar(c)
		}
	}
}

func findCallback(query string, key int) {
	if key == '\r' || key == 0x1b {
		findLastMatch = -1
		findDirection = 1
		if key == 0x1b {
			setSearchPattern("")
			updateAllSyntax(true)
		}
		return
	}
	if key == arrowRight || key == arrowDown {
		findDirection = 1
	} else if key == arrowLeft || key == arrowUp {
		findDirection = -1
	} else {
		findLastMatch = -1
		findDirection = 1
	}
	if query == "" {
		return
	}
	if findLastMatch == -1 {
		findDirection = 1
	}
	setSearchPattern(query)
	current := findLastMatch
	for i := 0; i < len(E.rows); i++ {
		current += findDirection
		if current == -1 {
			current = len(E.rows) - 1
		} else if current == len(E.rows) {
			current = 0
		}
		if E.searchRegexp != nil {
			p := E.searchRegexp.FindIndex(E.rows[current].s)
			if p != nil {
				findLastMatch = current
				E.cy = current
				E.cx = p[0]
				E.preferred = E.cx
				E.rowoff = len(E.rows)
				break
			}
		}
	}
	updateAllSyntax(true)
}

func find() {
	savedX, savedY, savedPref, savedCol, savedRow := E.cx, E.cy, E.preferred, E.coloff, E.rowoff
	q := prompt("/%s", findCallback)
	if q == "" {
		E.cx, E.cy, E.preferred, E.coloff, E.rowoff = savedX, savedY, savedPref, savedCol, savedRow
	}
}

func findNext(dir int) {
	if E.searchRegexp == nil || len(E.rows) == 0 {
		return
	}
	cur := E.cy
	curCol := E.cx
	if dir > 0 {
		curCol++
	} else {
		curCol--
	}
	for i := 0; i < len(E.rows); i++ {
		cur += dir
		if cur < 0 {
			cur = len(E.rows) - 1
		}
		if cur >= len(E.rows) {
			cur = 0
		}
		line := E.rows[cur].s
		if dir > 0 {
			startSearchAt := 0
			if cur == E.cy {
				startSearchAt = curCol
			}
			if startSearchAt < len(line) {
				m := E.searchRegexp.FindIndex(line[startSearchAt:])
				if m != nil {
					E.cy, E.cx, E.preferred = cur, startSearchAt+m[0], startSearchAt+m[0]
					updateAllSyntax(true)
					return
				}
			} else if cur != E.cy {
				// Search the whole line for next row
				m := E.searchRegexp.FindIndex(line)
				if m != nil {
					E.cy, E.cx, E.preferred = cur, m[0], m[0]
					updateAllSyntax(true)
					return
				}
			}
		} else {
			matches := E.searchRegexp.FindAllIndex(line, -1)
			if len(matches) > 0 {
				targetMatch := -1
				if cur == E.cy {
					for j := len(matches) - 1; j >= 0; j-- {
						if matches[j][0] <= curCol {
							targetMatch = j
							break
						}
					}
				} else {
					targetMatch = len(matches) - 1
				}
				if targetMatch != -1 {
					E.cy, E.cx, E.preferred = cur, matches[targetMatch][0], matches[targetMatch][0]
					updateAllSyntax(true)
					return
				}
			}
		}
	}
}

func selectWord() {
	if E.cy >= len(E.rows) || len(E.rows[E.cy].s) == 0 {
		return
	}
	r := E.rows[E.cy].s
	sx, ex := E.cx, E.cx
	for sx > 0 && isWordChar(r[sx-1]) {
		sx--
	}
	for ex < len(r)-1 && isWordChar(r[ex+1]) {
		ex++
	}
	E.mode = modeVisual
	E.selSY = E.cy
	E.selSX = sx
	E.cx = ex
}

func executeMenuAction(idx int) {
	switch idx {
	case 0: // Cut
		if E.mode == modeVisual || E.mode == modeVisualLine {
			yoink(E.selSX, E.selSY, E.cx, E.cy, E.mode == modeVisualLine)
			deleteRange(E.selSX, E.selSY, E.cx, E.cy)
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
		}
	case 1: // Copy
		if E.mode == modeVisual || E.mode == modeVisualLine {
			yoink(E.selSX, E.selSY, E.cx, E.cy, E.mode == modeVisualLine)
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
		}
	case 2: // Paste
		paste()
	case 3: // Select All
		selectAll()
	case 5: // Undo
		doUndo()
	case 6: // Redo
		doRedo()
	}
}

func handleSubstitute(cmd string) {
	allLines := false
	sCmd := cmd
	if strings.HasPrefix(cmd, "%") {
		allLines = true
		sCmd = cmd[1:]
	}

	if !strings.HasPrefix(sCmd, "s") {
		return
	}

	rest := sCmd[1:]
	if len(rest) < 3 {
		setStatus("Invalid substitute command")
		return
	}

	delimiter := rest[0]
	var parts []string
	var current strings.Builder
	escaped := false
	for i := 1; i < len(rest); i++ {
		if escaped {
			current.WriteByte(rest[i])
			escaped = false
		} else if rest[i] == '\\' {
			escaped = true
		} else if rest[i] == delimiter {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(rest[i])
		}
	}
	parts = append(parts, current.String())

	if len(parts) < 2 {
		setStatus("Invalid substitute command")
		return
	}

	pattern := parts[0]
	replacement := parts[1]
	flags := ""
	if len(parts) > 2 {
		flags = parts[2]
	}

	global := strings.Contains(flags, "g")

	startRow := 0
	endRow := len(E.rows) - 1
	if !allLines {
		startRow = E.cy
		endRow = E.cy
	}

	if startRow < 0 || startRow >= len(E.rows) {
		return
	}

	saveUndo()
	madeChanges := false
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		setStatus("Invalid regex: %v", err)
		return
	}

	for y := startRow; y <= endRow; y++ {
		line := string(E.rows[y].s)
		var newLine string
		if global {
			newLine = re.ReplaceAllString(line, replacement)
		} else {
			// Replace only first occurrence
			found := false
			newLine = re.ReplaceAllStringFunc(line, func(match string) string {
				if found {
					return match
				}
				found = true
				return re.ReplaceAllString(match, replacement)
			})
		}

		if newLine != line {
			E.rows[y] = duplicateRow(E.rows[y])
			E.rows[y].s = []byte(newLine)
			updateSyntax(E.rows[y], false)
			madeChanges = true
		}
	}
	if madeChanges {
		E.dirty = true
		setStatus("Substitutions complete")
	} else {
		setStatus("Pattern not found")
	}
}

func prompt(p string, cb func(string, int)) string {
	buf := make([]byte, 0, 128)
	for {
		setStatus(p, string(buf))
		refreshScreen()
		c := readKey()
		switch c {
		case delKey, backspace, 127, 8:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case 0x1b:
			setStatus("")
			if cb != nil {
				cb(string(buf), c)
			}
			return ""
		case '\r':
			if len(buf) > 0 {
				setStatus("")
				if cb != nil {
					cb(string(buf), c)
				}
				return string(buf)
			}
		default:
			if c >= 32 && c < 128 {
				buf = append(buf, byte(c))
			}
		}
		if cb != nil {
			cb(string(buf), c)
		}
	}
}

func applyMotionKey(key int, count int) bool {
	changed := false
	for i := 0; i < count; i++ {
		px, py := E.cx, E.cy
		switch key {
		case 'h':
			moveLeftNoWrap()
		case 'j':
			moveCursor(arrowDown)
		case 'k':
			moveCursor(arrowUp)
		case 'l':
			moveRightNoWrap()
		case arrowLeft, arrowRight, arrowUp, arrowDown:
			moveCursor(key)
		case shiftUp:
			movePreviousParagraph()
		case shiftDown:
			moveNextParagraph()
		case shiftLeft:
			moveWordBackward(false)
		case shiftRight:
			moveWordForward(false)
		case '0':
			moveLineStart()
		case '^':
			moveFirstNonWhitespace()
		case '$':
			moveLineEnd()
		case 'w':
			moveWordForward(false)
		case 'W':
			moveWordForward(true)
		case 'b':
			moveWordBackward(false)
		case 'B':
			moveWordBackward(true)
		case 'e':
			moveWordEnd(false)
		case 'E':
			moveWordEnd(true)
		case '{':
			movePreviousParagraph()
		case '}':
			moveNextParagraph()
		default:
			return changed
		}
		if E.cx != px || E.cy != py {
			changed = true
		}
	}
	return changed
}

func operatorExclusiveMotion(m int) bool {
	switch m {
	case 'w', 'W', 'b', 'B', '0', '^', '{', '}', 'h', 'j', 'k', 'l', 'g', 'G':
		return true
	}
	return false
}

func posBefore(ax, ay, bx, by int) bool {
	if ay != by {
		return ay < by
	}
	return ax < bx
}

func prevPos(x, y int) (int, int, bool) {
	if len(E.rows) == 0 || y < 0 || y >= len(E.rows) {
		return 0, 0, false
	}
	if x > 0 {
		return utf8PrevBoundary(E.rows[y].s, x), y, true
	}
	if y == 0 {
		return 0, 0, false
	}
	py := y - 1
	if len(E.rows[py].s) == 0 {
		return 0, py, true
	}
	return utf8PrevBoundary(E.rows[py].s, len(E.rows[py].s)), py, true
}

func isDelimiter(c byte) bool {
	return strings.ContainsRune(" \t()[]{}<>\"'`", rune(c))
}

func findTextObject(obj int, inner bool) (sx, sy, ex, ey int, ok bool) {
	if len(E.rows) == 0 {
		return 0, 0, 0, 0, false
	}
	line := E.rows[E.cy].s
	if len(line) == 0 {
		return 0, 0, 0, 0, false
	}

	switch obj {
	case 'w', 'W':
		big := obj == 'W'
		sx, ex = E.cx, E.cx
		// Find start of word
		for sx > 0 {
			prev := line[sx-1]
			if big {
				if prev == ' ' || prev == '\t' {
					break
				}
			} else {
				if !isWordChar(prev) {
					break
				}
			}
			sx--
		}
		// Find end of word
		for ex < len(line)-1 {
			next := line[ex+1]
			if big {
				if next == ' ' || next == '\t' {
					break
				}
			} else {
				if !isWordChar(next) {
					break
				}
			}
			ex++
		}
		if !inner {
			// Include trailing whitespace
			for ex < len(line)-1 && (line[ex+1] == ' ' || line[ex+1] == '\t') {
				ex++
			}
		}
		return sx, E.cy, ex, E.cy, true

	case '"', '\'', '`':
		delim := byte(obj)
		sx, ex = -1, -1
		// Search backwards for the opening delimiter
		for x := E.cx; x >= 0; x-- {
			if line[x] == delim {
				sx = x
				break
			}
		}
		// Search forwards for the closing delimiter
		for x := E.cx; x < len(line); x++ {
			if line[x] == delim {
				ex = x
				break
			}
		}
		if sx == -1 || ex == -1 || sx == ex {
			// If not found surrounding, try finding on current line
			sx, ex = -1, -1
			for x := 0; x < len(line); x++ {
				if line[x] == delim {
					if sx == -1 {
						sx = x
					} else {
						ex = x
						if E.cx >= sx && E.cx <= ex {
							break
						}
						sx, ex = x, -1
					}
				}
			}
		}
		if sx != -1 && ex != -1 {
			if inner {
				return sx + 1, E.cy, ex - 1, E.cy, true
			}
			return sx, E.cy, ex, E.cy, true
		}

	case '(', ')', 'b', '[', ']', '{', '}', 'B', '<', '>':
		open, close := byte(0), byte(0)
		switch obj {
		case '(', ')', 'b':
			open, close = '(', ')'
		case '[', ']':
			open, close = '[', ']'
		case '{', '}', 'B':
			open, close = '{', '}'
		case '<', '>':
			open, close = '<', '>'
		}
		
		// Find surrounding brackets with nesting support
		sy, sx = E.cy, E.cx
		depth := 0
		foundOpen := false
		for sy >= 0 {
			line := E.rows[sy].s
			start := sx
			if sy < E.cy {
				start = len(line) - 1
			}
			for x := start; x >= 0; x-- {
				if line[x] == close {
					depth++
				} else if line[x] == open {
					if depth == 0 {
						sx, foundOpen = x, true
						break
					}
					depth--
				}
			}
			if foundOpen {
				break
			}
			sy--
		}
		if !foundOpen {
			return 0, 0, 0, 0, false
		}

		ey, ex = E.cy, E.cx
		depth = 0
		foundClose := false
		for ey < len(E.rows) {
			line := E.rows[ey].s
			start := ex
			if ey > E.cy {
				start = 0
			}
			for x := start; x < len(line); x++ {
				if line[x] == open {
					depth++
				} else if line[x] == close {
					if depth == 0 {
						ex, foundClose = x, true
						break
					}
					depth--
				}
			}
			if foundClose {
				break
			}
			ey++
		}
		if foundClose {
			if inner {
				isx, isy, iex, iey := sx, sy, ex, ey
				isx, isy, _ = nextPos(isx, isy)
				iex, iey, _ = prevPos(iex, iey)
				return isx, isy, iex, iey, true
			}
			return sx, sy, ex, ey, true
		}
	}
	return 0, 0, 0, 0, false
}

func nextPos(x, y int) (int, int, bool) {
	if len(E.rows) == 0 || y < 0 || y >= len(E.rows) {
		return 0, 0, false
	}
	if x < len(E.rows[y].s) {
		return utf8NextBoundary(E.rows[y].s, x), y, true
	}
	if y >= len(E.rows)-1 {
		return x, y, false
	}
	return 0, y + 1, true
}

func handleOperator(op int, count int) bool {
	if len(E.rows) == 0 {
		if op == 'c' {
			E.mode = modeInsert
			setStatus("-- INSERT --")
			startRecording(op)
		}
		return true
	}
	m := readKey()
	if m == resizeEvent {
		return true
	}

	if m == 'i' || m == 'a' {
		obj := readKey()
		if obj == resizeEvent {
			return true
		}
		sx, sy, ex, ey, ok := findTextObject(obj, m == 'i')
		if ok {
			if op != 'y' {
				startRecording(op)
				E.currentChange = append(E.currentChange, m, obj)
			}
			yoink(sx, sy, ex, ey, false)
			if op != 'y' {
				deleteRange(sx, sy, ex, ey)
				if op == 'c' {
					E.mode = modeInsert
					setStatus("-- INSERT --")
				} else {
					stopRecording()
				}
			}
			E.SelectedRegister = '"'
			return true
		}
		E.SelectedRegister = '"'
		return true
	}

	if m == op {
		if op != 'y' {
			startRecording(op)
			E.currentChange = append(E.currentChange, m)
		}
		sy := E.cy
		ey := min(E.cy+count-1, len(E.rows)-1)
		yoink(0, sy, 0, ey, true)
		if op != 'y' {
			saveUndo()
			for i := 0; i <= ey-sy; i++ {
				delRow(sy)
			}
			if len(E.rows) == 0 {
				insertRow(0, nil)
			}
			E.cy = min(sy, len(E.rows)-1)
			E.cx, E.preferred = 0, 0
		}
		if op == 'c' {
			E.mode = modeInsert
			setStatus("-- INSERT --")
		} else if op != 'y' {
			stopRecording()
		}
		E.SelectedRegister = '"'
		return true
	}

	startX, startY := E.cx, E.cy
	usedMotion := m
	switch m {
	case 'g':
		n := readKey()
		if n != 'g' {
			return true
		}
		moveFileStart()
		usedMotion = 'g' // Simplify for exclusive check, but actually gg is the motion
	case 'G':
		if count > 1 {
			moveToLine(count)
		} else {
			moveFileEnd()
		}
	case 'f', 'F', 't', 'T':
		n := readKey()
		if n < 32 || n > 255 || n == 127 {
			return true
		}
		dir := 1
		if m == 'F' || m == 'T' {
			dir = -1
		}
		till := m == 't' || m == 'T'
		for i := 0; i < count; i++ {
			if !findChar(byte(n), dir, till) {
				break
			}
		}
	default:
		if !applyMotionKey(m, count) {
			return true
		}
	}

	destX, destY := E.cx, E.cy
	if destX == startX && destY == startY {
		return true
	}

	var sx, sy, ex, ey int
	if posBefore(startX, startY, destX, destY) {
		sx, sy = startX, startY
		if operatorExclusiveMotion(usedMotion) {
			px, py, ok := prevPos(destX, destY)
			if !ok {
				return true
			}
			ex, ey = px, py
		} else {
			ex, ey = destX, destY
		}
	} else {
		sx, sy = destX, destY
		if operatorExclusiveMotion(usedMotion) {
			px, py, ok := prevPos(startX, startY)
			if !ok {
				return true
			}
			ex, ey = px, py
		} else {
			ex, ey = startX, startY
		}
	}

	yoink(sx, sy, ex, ey, false)
	if op != 'y' {
		deleteRange(sx, sy, ex, ey)
		if op == 'c' {
			E.mode = modeInsert
			setStatus("-- INSERT --")
		} else {
			stopRecording()
		}
	} else {
		E.cx, E.cy, E.preferred = startX, startY, startX
	}
	E.SelectedRegister = '"'
	return true
}

func processKeypress() bool {
	c := readKey()
	if c == -1 {
		return false
	}
	if c == resizeEvent {
		return true
	}
	if E.menuOpen && c != mouseEvent {
		E.menuOpen = false
		if c == 0x1b {
			return true
		}
	}
	if c == mouseEvent {
		return handleMouse()
	}
	if c == pasteEvent {
		if len(E.pasteBuffer) > 0 {
			for i := 0; i < len(E.pasteBuffer); i++ {
				ch := E.pasteBuffer[i]
				if ch == '\r' || ch == '\n' {
					insertNewline()
					if ch == '\r' && i+1 < len(E.pasteBuffer) && E.pasteBuffer[i+1] == '\n' {
						i++
					}
				} else {
					insertChar(ch)
				}
			}
			E.pasteBuffer = nil
		}
		return true
	}
	if E.mode == modeVisual || E.mode == modeVisualLine {
		switch c {
		case 'i', 'a':
			obj := readKey()
			if obj == resizeEvent {
				return true
			}
			sx, sy, ex, ey, ok := findTextObject(obj, c == 'i')
			if ok {
				E.selSX, E.selSY = sx, sy
				E.cx, E.cy = ex, ey
				E.preferred = E.cx
				E.mode = modeVisual
			}
			return true
		}
	}
	if E.mode == modeInsert {
		switch c {
		case '\r':
			insertNewline()
		case '\t':
			insertChar(' ')
			insertChar(' ')
			insertChar(' ')
			insertChar(' ')
		case 0x1b:
			E.mode = modeNormal
			if E.cx > 0 {
				E.cx--
			}
			setStatus("")
			stopRecording()
		case backspace, 127, 8:
			delChar()
		case delKey:
			moveCursor(arrowRight)
			delChar()
		case arrowLeft, arrowRight, arrowUp, arrowDown:
			moveCursor(c)
		case shiftUp:
			movePreviousParagraph()
		case shiftDown:
			moveNextParagraph()
		case shiftLeft:
			moveWordBackward(false)
		case shiftRight:
			moveWordForward(false)
		default:
			if c >= 32 && c <= 255 && c != 127 {
				insertChar(byte(c))
			}
		}
		return true
	}

	if c >= '1' && c <= '9' {
		E.countPrefix = E.countPrefix*10 + (c - '0')
		return true
	}
	if c == '0' && E.countPrefix > 0 {
		E.countPrefix *= 10
		return true
	}
	count := E.countPrefix
	if count <= 0 {
		count = 1
	}
	usedCount := E.countPrefix > 0
	E.countPrefix = 0

	switch c {
	case 'i':
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'a':
		if E.cy >= 0 && E.cy < len(E.rows) && E.cx < len(E.rows[E.cy].s) {
			E.cx = utf8NextBoundary(E.rows[E.cy].s, E.cx)
			if E.cx > len(E.rows[E.cy].s) {
				E.cx = len(E.rows[E.cy].s)
			}
		}
		E.preferred = E.cx
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'I':
		moveFirstNonWhitespace()
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'A':
		moveLineEnd()
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'o':
		if len(E.rows) == 0 {
			insertRow(0, nil)
			E.cy, E.cx = 0, 0
		} else {
			E.cy = min(E.cy, len(E.rows)-1)
			E.cx = len(E.rows[E.cy].s)
		}
		insertNewline()
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'O':
		if len(E.rows) == 0 {
			insertRow(0, nil)
			E.cy, E.cx = 0, 0
		} else {
			E.cy = min(E.cy, len(E.rows)-1)
			E.cx = 0
		}
		insertNewline()
		E.cy--
		E.cx, E.preferred = 0, 0
		E.mode = modeInsert
		E.selSX, E.selSY = -1, -1
		setStatus("-- INSERT --")
		startRecording(c)
	case 'v':
		if E.mode == modeVisual {
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
		} else {
			E.mode = modeVisual
			E.selSX, E.selSY = E.cx, E.cy
		}
	case 'V':
		if E.mode == modeVisualLine {
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
		} else {
			E.mode = modeVisualLine
			E.selSX, E.selSY = 0, E.cy
		}
	case 'u':
		if E.mode == modeVisual || E.mode == modeVisualLine {
			changeCase(false)
		} else {
			for i := 0; i < count; i++ {
				doUndo()
			}
		}
	case 18:
		for i := 0; i < count; i++ {
			doRedo()
		}
	case 1:
		incrementNumber(count)
	case 24:
		incrementNumber(-count)
	case 'U':
		if E.mode == modeVisual || E.mode == modeVisualLine {
			changeCase(true)
		}
	case 'Z':
		if m := readKey(); m == 'Z' {
			saveFile()
			E.Screen.Fini()
			os.Exit(0)
		} else if m == 'Q' {
			E.Screen.Fini()
			os.Exit(0)
		}
	case 'y', 'd', 'x', 'c':
		sx, sy, ex, ey := E.selSX, E.selSY, E.cx, E.cy
		isLine := E.mode == modeVisualLine
		if E.mode == modeNormal {
			sx, sy, ex, ey = E.cx, E.cy, E.cx, E.cy
			if c == 'd' {
				// Special case: 'dd' in Vim deletes line. Helix uses 'x' to select line.
				// We will keep 'handleOperator' for now to support 'dd', 'dw' etc.
				// but simplify the Visual mode branch.
				return handleOperator(c, count)
			}
			if c == 'x' {
				// 'x' is already a single-char delete in Normal mode
				ex = min(ex+count-1, len(E.rows[ey].s)-1)
			}
		}

		startRecording(c)
		yoink(sx, sy, ex, ey, isLine)
		if c != 'y' {
			deleteRange(sx, sy, ex, ey)
		}
		
		if c == 'c' {
			E.mode = modeInsert
			setStatus("-- INSERT --")
		} else {
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
			stopRecording()
		}
		E.SelectedRegister = '"'
	case 'p':
		for i := 0; i < count; i++ {
			paste()
		}
		E.SelectedRegister = '"'
	case ctrlShiftC:
		if E.mode == modeVisual || E.mode == modeVisualLine {
			yoink(E.selSX, E.selSY, E.cx, E.cy, E.mode == modeVisualLine)
			E.mode = modeNormal
			E.selSX, E.selSY = -1, -1
		}
		return true
	case 3:
		if E.dirty && E.quitWarnRemaining > 0 {
			setStatus("WARNING!!! File has unsaved changes. Press Ctrl-C %d more times to quit.", E.quitWarnRemaining)
			E.quitWarnRemaining--
			return true
		}
		E.Screen.Fini()
		os.Exit(0)
	case ':':
		rawCmd := prompt(":%s", nil)
		cmd := strings.TrimSpace(rawCmd)
		switch {
		case cmd == "q":
			if E.dirty {
				setStatus("No write since last change (add ! to override)")
			} else {
				E.Screen.Fini()
				os.Exit(0)
			}
		case cmd == "q!" || cmd == "qa!":
			E.Screen.Fini()
			os.Exit(0)
		case cmd == "w":
			saveFile()
		case cmd == "wq":
			saveFile()
			E.Screen.Fini()
			os.Exit(0)
		case cmd == "h" || cmd == "help":
			if E.dirty {
				setStatus("No write since last change (add ! to override)")
				break
			}
			if openFile("help.txt") {
				E.cx, E.cy, E.preferred = 0, 0, 0
				E.rowoff, E.coloff = 0, 0
			}
		case strings.HasPrefix(cmd, "e "):
			if E.dirty {
				setStatus("No write since last change (add ! to override)")
				break
			}
			target := strings.TrimSpace(strings.TrimPrefix(cmd, "e"))
			if target == "" {
				setStatus("No file name")
				break
			}
			if openFile(target) {
				E.cx, E.cy, E.preferred = 0, 0, 0
				E.rowoff, E.coloff = 0, 0
			}
		case strings.HasPrefix(cmd, "s") || strings.HasPrefix(cmd, "%s"):
			handleSubstitute(cmd)
		default:
			if cmd == "$" {
				moveToLine(len(E.rows))
				break
			}
			if cmd != "" {
				if n, err := strconv.Atoi(cmd); err == nil {
					moveToLine(n)
					break
				}
			}
			if cmd != "" {
				setStatus("Not an editor command: %s", cmd)
			}
		}
	case '/':
		find()
	case 'n':
		for i := 0; i < count; i++ {
			findNext(1)
		}
	case 'N':
		for i := 0; i < count; i++ {
			findNext(-1)
		}
	case '"':
		setStatus("\"")
		r := readKey()
		if r >= 32 && r <= 255 {
			E.SelectedRegister = r
			setStatus("\"%c", rune(r))
		}
		return true
	case 'h':
		_ = applyMotionKey('h', count)
	case 'j':
		_ = applyMotionKey('j', count)
	case 'k':
		_ = applyMotionKey('k', count)
	case 'l':
		_ = applyMotionKey('l', count)
	case arrowLeft, arrowRight, arrowUp, arrowDown,
		shiftLeft, shiftRight, shiftUp, shiftDown:
		_ = applyMotionKey(c, count)
	case homeKey:
		E.cx = 0
	case '0':
		moveLineStart()
	case '^':
		moveFirstNonWhitespace()
	case endKey:
		if E.cy >= 0 && E.cy < len(E.rows) {
			E.cx = len(E.rows[E.cy].s)
		}
	case '$':
		moveLineEnd()
	case pageUp, pageDown:
		if c == pageUp {
			E.cy = E.rowoff
		} else {
			E.cy = E.rowoff + E.screenRows - 1
			if E.cy > len(E.rows) {
				E.cy = len(E.rows)
			}
		}
		for i := 0; i < E.screenRows; i++ {
			if c == pageUp {
				moveCursor(arrowUp)
			} else {
				moveCursor(arrowDown)
			}
		}
	case 'w':
		_ = applyMotionKey('w', count)
	case 'W':
		_ = applyMotionKey('W', count)
	case 'b':
		_ = applyMotionKey('b', count)
	case 'B':
		_ = applyMotionKey('B', count)
	case 'e':
		_ = applyMotionKey('e', count)
	case 'E':
		_ = applyMotionKey('E', count)
	case 'g':
		if readKey() == 'g' {
			if usedCount {
				moveToLine(count)
			} else {
				moveFileStart()
			}
		}
	case 'G':
		if usedCount {
			moveToLine(count)
		} else {
			moveFileEnd()
		}
	case 'm':
		m := readKey()
		if m >= 'a' && m <= 'z' {
			i := m - 'a'
			E.markSet[i] = true
			E.marksX[i] = E.cx
			E.marksY[i] = E.cy
		}
	case '\'':
		m := readKey()
		if m >= 'a' && m <= 'z' {
			i := m - 'a'
			if E.markSet[i] {
				if len(E.rows) == 0 {
					E.cy, E.cx = 0, 0
				} else {
					E.cy = min(max(0, E.marksY[i]), len(E.rows)-1)
					E.cx = min(E.marksX[i], len(E.rows[E.cy].s))
				}
				E.preferred = E.cx
			} else {
				setStatus("E20: Mark not set")
			}
		}
	case '%':
		matchBracket()
	case '{':
		_ = applyMotionKey('{', count)
	case '}':
		_ = applyMotionKey('}', count)
	case '>':
		if E.mode == modeVisual || E.mode == modeVisualLine {
			startRecording(c)
			indentSelection(true)
			stopRecording()
		} else {
			if readKey() == '>' {
				startRecording(c)
				E.currentChange = append(E.currentChange, '>')
				saveUndo()
				for i := 0; i < count; i++ {
					y := E.cy + i
					if y < len(E.rows) {
						E.rows[y] = duplicateRow(E.rows[y])
						E.rows[y].s = append([]byte("    "), E.rows[y].s...)
						updateSyntax(E.rows[y], false)
					}
				}
				E.dirty = true
				stopRecording()
			}
		}
	case '<':
		if E.mode == modeVisual || E.mode == modeVisualLine {
			startRecording(c)
			indentSelection(false)
			stopRecording()
		} else {
			if readKey() == '<' {
				startRecording(c)
				E.currentChange = append(E.currentChange, '<')
				saveUndo()
				for i := 0; i < count; i++ {
					y := E.cy + i
					if y < len(E.rows) {
						E.rows[y] = duplicateRow(E.rows[y])
						trim := 0
						for trim < 4 && trim < len(E.rows[y].s) && E.rows[y].s[trim] == ' ' {
							trim++
						}
						if trim > 0 {
							E.rows[y].s = E.rows[y].s[trim:]
							updateSyntax(E.rows[y], false)
						}
					}
				}
				E.dirty = true
				stopRecording()
			}
		}
	case 'f', 'F', 't', 'T':
		n := readKey()
		if n >= 32 && n <= 255 && n != 127 {
			dir := 1
			if c == 'F' || c == 'T' {
				dir = -1
			}
			till := c == 't' || c == 'T'
			found := false
			for i := 0; i < count; i++ {
				if !findChar(byte(n), dir, till) {
					break
				}
				found = true
			}
			if found {
				setStatus("Found %c at %d,%d", n, E.cy+1, E.cx+1)
			}
		}
	case ';':
		for i := 0; i < count; i++ {
			repeatCharSearch(false)
		}
	case ',':
		for i := 0; i < count; i++ {
			repeatCharSearch(true)
		}
	case 0x1b:
		E.mode = modeNormal
		E.selSX, E.selSY = -1, -1
		setStatus("")
	}
	E.quitWarnRemaining = 1
	return true
}
