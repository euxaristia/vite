package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

func writeCursorPos(b *bytes.Buffer, row, col int) {
	b.WriteString("\x1b[")
	n := strconv.AppendInt(cursorNumBuf[:0], int64(row), 10)
	b.Write(n)
	b.WriteByte(';')
	n = strconv.AppendInt(cursorNumBuf[:0], int64(col), 10)
	b.Write(n)
	b.WriteByte('H')
}

func getWindowSize() (int, int) {
	ws, err := ioctlGetWinsize(int(os.Stdout.Fd()), syscall.TIOCGWINSZ)
	if err == nil && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Row), int(ws.Col)
	}
	return 24, 80
}

func updateWindowSize() {
	r, c := getWindowSize()
	E.screenRows = r - 2
	if E.screenRows < 1 {
		E.screenRows = 1
	}
	E.screenCols = c
}

func runeDisplayWidth(r rune) int {
	if r < 0x20 || (r >= 0x7F && r < 0xA0) {
		return 0
	}
	if (r >= 0x0300 && r <= 0x036F) || (r >= 0x1AB0 && r <= 0x1AFF) ||
		(r >= 0x1DC0 && r <= 0x1DFF) || (r >= 0x20D0 && r <= 0x20FF) ||
		(r >= 0xFE20 && r <= 0xFE2F) || r == 0x200D ||
		(r >= 0xFE00 && r <= 0xFE0F) {
		return 0
	}
	if (r >= 0x1100 && r <= 0x115F) || (r >= 0x2329 && r <= 0x232A) ||
		(r >= 0x2E80 && r <= 0xA4CF) || (r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0xFE10 && r <= 0xFE19) ||
		(r >= 0xFE30 && r <= 0xFE6F) || (r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) || (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF) {
		return 2
	}
	return 1
}

var syntaxColorLUT = [...]string{
	hlNormal:      "\x1b[37m",
	hlComment:     "\x1b[32m",
	hlKeyword1:    "\x1b[33m",
	hlKeyword2:    "\x1b[36m",
	hlString:      "\x1b[35m",
	hlNumber:      "\x1b[31m",
	hlMatch:       "\x1b[34m",
	hlMatchCursor: "\x1b[33m",
}

func drawRows(b *bytes.Buffer) {
	g := gutterWidth()
	gcols := 0
	if g > 0 {
		gcols = g + 1
	}
	textCols := E.screenCols - gcols
	if textCols < 1 {
		textCols = 1
	}
	hasSelection := E.mode == modeVisual || E.mode == modeVisualLine
	lineSelection := E.mode == modeVisualLine
	sy, ey, sx, ex := 0, 0, 0, 0
	if hasSelection {
		sy, ey, sx, ex = E.selSY, E.cy, E.selSX, E.cx
		if sy > ey || (sy == ey && sx > ex) {
			sy, ey = ey, sy
			sx, ex = ex, sx
		}
	}
	var lineNumBuf []byte
	offsetsChanged := E.rowoff != E.lastRowoff || E.coloff != E.lastColoff
	for y := 0; y < E.screenRows; y++ {
		fr := y + E.rowoff
		if !hasSelection && !offsetsChanged && fr < len(E.rows) && y < len(E.lastRows) && E.rows[fr] == E.lastRows[y] {
			b.WriteString("\x1b[B") // Move cursor down one line
			continue
		}
		if fr >= len(E.rows) {
			if len(E.rows) == 0 && y >= E.screenRows/3 && y < E.screenRows/3+len(welcomeLines) {
				b.WriteString("\x1b[2m~\x1b[m")
				msg := welcomeLines[y-E.screenRows/3]
				if len(msg) > textCols {
					msg = msg[:textCols]
				}
				padding := (textCols - len(msg)) / 2
				for i := 0; i < padding; i++ {
					b.WriteByte(' ')
				}
				b.WriteString(msg)
			} else {
				b.WriteString("\x1b[2m~\x1b[m")
			}
		} else {
			if g > 0 {
				b.WriteString("\x1b[2m")
				lineNumBuf = strconv.AppendInt(lineNumBuf[:0], int64(fr+1), 10)
				for i := 0; i < g-len(lineNumBuf); i++ {
					b.WriteByte(' ')
				}
				b.Write(lineNumBuf)
				b.WriteString(" \x1b[m")
			}
			rowData := E.rows[fr]
			updateSyntax(rowData, false)
			line := rowData.s
			start := utf8SnapBoundary(line, E.coloff)
			if start > len(line) {
				start = len(line)
			}
			hl := rowData.hl[start:]
			visible := line[start:]
			curColorSeq := ""
			curSelected := false
			curReverse := 0
			rowInSelection := hasSelection && fr >= sy && fr <= ey
			drawnCols := 0
			for i := 0; i < len(visible) && drawnCols < textCols; i++ {
				sel := false
				if rowInSelection {
					x := i + start
					if lineSelection {
						sel = true
					} else if fr >= sy && fr <= ey {
						if sy == ey {
							sel = x >= sx && x <= ex
						} else if fr == sy {
							sel = x >= sx
						} else if fr == ey {
							sel = x <= ex
						} else {
							sel = true
						}
					}
				}
				if sel != curSelected {
					if sel {
						b.WriteString("\x1b[48;5;242m")
					} else {
						b.WriteString("\x1b[49m")
					}
					curSelected = sel
				}
				h := hl[i]
				reverse := 0
				if h == hlMatch {
					reverse = 1
				} else if h == hlMatchCursor {
					reverse = 2
				}
				prevReverse := curReverse
				if reverse != curReverse {
					curReverse = reverse
					if curReverse == 1 {
						b.WriteString("\x1b[7m\x1b[48;5;94m")
					} else if curReverse == 2 {
						b.WriteString("\x1b[7m\x1b[48;5;220m")
					} else {
						b.WriteString("\x1b[27m")
					}
				}
				if curReverse == 0 {
					if prevReverse != 0 {
						curSelected = !sel
					}
					if sel != curSelected {
						if sel {
							b.WriteString("\x1b[48;5;242m")
						} else {
							b.WriteString("\x1b[49m")
						}
						curSelected = sel
					}
					seq := syntaxColorLUT[hlNormal]
					if int(h) < len(syntaxColorLUT) {
						seq = syntaxColorLUT[h]
					}
					if seq != curColorSeq {
						b.WriteString(seq)
						curColorSeq = seq
					}
				}
				if visible[i] == '\t' {
					tabCols := 8 - ((gcols + drawnCols) % 8)
					if tabCols <= 0 {
						tabCols = 8
					}
					remaining := textCols - drawnCols
					if tabCols > remaining {
						tabCols = remaining
					}
					for s := 0; s < tabCols; s++ {
						b.WriteByte(' ')
					}
					drawnCols += tabCols
					continue
				}
				b.WriteByte(safeTermByte(visible[i]))
				drawnCols++
			}
			b.WriteString("\x1b[27m\x1b[39m\x1b[49m")
		}
		b.WriteString("\x1b[K\r\n")
	}
}

func drawStatusBar(b *bytes.Buffer) {
	b.WriteString("\x1b[48;5;250m\x1b[38;5;240m")
	left := " [No Name]"
	if E.filename != "" {
		left = " " + E.filename
	}
	if E.dirty {
		left += " [+]"
	}
	if E.gitStatus != "" {
		left += " [" + E.gitStatus + "]"
	}
	left = safeTermString(left)
	pos := "All"
	if len(E.rows) > 0 {
		if E.rowoff == 0 {
			pos = "Top"
		} else if E.rowoff+E.screenRows >= len(E.rows) {
			pos = "Bot"
		} else {
			pos = strconv.Itoa((E.rowoff*100)/max(1, len(E.rows)-E.screenRows)) + "%"
		}
	}

	rx := 0
	if E.cy >= 0 && E.cy < len(E.rows) {
		row := E.rows[E.cy].s
		for i := 0; i < E.cx && i < len(row); {
			if row[i] == '\t' {
				rx += 8 - (rx % 8)
				i++
				continue
			}
			r, n := utf8.DecodeRune(row[i:])
			if n <= 0 {
				n = 1
			}
			w := runeDisplayWidth(r)
			if w < 0 {
				w = 1
			}
			rx += w
			i += n
		}
	}
	var loc [48]byte
	locB := loc[:0]
	if len(E.rows) > 0 {
		locB = strconv.AppendInt(locB, int64(E.cy+1), 10)
		locB = append(locB, ',')
		locB = strconv.AppendInt(locB, int64(E.cx+1), 10)
		locB = append(locB, '-')
		locB = strconv.AppendInt(locB, int64(rx+1), 10)
	} else {
		locB = append(locB, "0,0-1"...)
	}
	locField := len(locB)
	if locField < 14 {
		locField = 14
	}
	rightLen := 1 + locField + 1 + len(pos)
	if len(left) > E.screenCols-rightLen {
		left = left[:max(0, E.screenCols-rightLen)]
	}
	b.WriteString(left)
	pad := E.screenCols - rightLen - len(left)
	for i := 0; i < pad; i++ {
		b.WriteByte(' ')
	}
	b.WriteByte(' ')
	b.Write(locB)
	for i := len(locB); i < 14; i++ {
		b.WriteByte(' ')
	}
	b.WriteByte(' ')
	b.WriteString(pos)
	b.WriteString("\x1b[m\r\n")
}

func drawMessageBar(b *bytes.Buffer) {
	b.WriteString("\x1b[K")
	if E.statusmsg != "" && time.Since(E.statusTime) < 5*time.Second {
		msg := safeTermString(E.statusmsg)
		if len(msg) > E.screenCols {
			msg = msg[:E.screenCols]
		}
		b.WriteString(msg)
		return
	}
	switch E.mode {
	case modeInsert:
		b.WriteString("-- INSERT --")
	case modeVisual:
		b.WriteString("-- VISUAL --")
	case modeVisualLine:
		b.WriteString("-- VISUAL LINE --")
	}
}

var menuItems = []string{
	" Cut       ",
	" Copy      ",
	" Paste     ",
	" Select All ",
	"----------- ",
	" Undo      ",
	" Redo      ",
}

var contextMenuW int
var contextMenuHLine string
var contextMenuLabels []string
var contextMenuTopBorder string
var contextMenuBottomBorder string

func initContextMenuMetrics() {
	w := 1
	for _, item := range menuItems {
		if len(item) > w {
			w = len(item)
		}
	}
	contextMenuW = w
	contextMenuHLine = strings.Repeat("─", w)
	contextMenuTopBorder = "\x1b[48;5;235m\x1b[38;5;239m┌" + contextMenuHLine + "┐"
	contextMenuBottomBorder = "\x1b[48;5;235m\x1b[38;5;239m└" + contextMenuHLine + "┘\x1b[m"
	contextMenuLabels = make([]string, len(menuItems))
	for i, item := range menuItems {
		label := item
		if i == 4 {
			label = contextMenuHLine
		} else if len(label) < w {
			label += strings.Repeat(" ", w-len(label))
		} else if len(label) > w {
			label = label[:w]
		}
		contextMenuLabels[i] = label
	}
}

var welcomeLines = []string{
	"VIDERE v0.1.0",
	"",
	"videre is open source and freely distributable",
	"https://github.com/euxaristia/videre",
	"",
	"type  :q<Enter>               to exit         ",
	"type  :wq<Enter>              save and exit   ",
	"",
	"Maintainer: euxaristia",
}

func drawContextMenu(b *bytes.Buffer) {
	if !E.menuOpen {
		return
	}
	x := E.menuX
	y := E.menuY
	innerW := contextMenuW
	menuW := innerW + 2
	menuH := len(menuItems) + 2
	if x+menuW > E.screenCols {
		x = E.screenCols - menuW
	}
	if y+menuH > E.screenRows {
		y = E.screenRows - menuH
	}
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	writeCursorPos(b, y, x)
	b.WriteString(contextMenuTopBorder)
	for i := range menuItems {
		writeCursorPos(b, y+i+1, x)
		label := contextMenuLabels[i]
		if i == E.menuSelected {
			b.WriteString("\x1b[48;5;24m\x1b[38;5;255m│")
			b.WriteString(label)
			b.WriteString("│")
		} else {
			b.WriteString("\x1b[48;5;235m\x1b[38;5;239m│\x1b[38;5;252m")
			if i == 4 {
				b.WriteString("\x1b[38;5;239m")
			}
			b.WriteString(label)
			b.WriteString("\x1b[38;5;239m│")
		}
	}
	writeCursorPos(b, y+len(menuItems)+1, x)
	b.WriteString(contextMenuBottomBorder)
}

func scroll() {
	if E.rowoff < 0 {
		E.rowoff = 0
	}
	if E.coloff < 0 {
		E.coloff = 0
	}
	if len(E.rows) == 0 {
		E.cx = 0
		E.cy = 0
		E.preferred = 0
		return
	}
	if E.cy < 0 {
		E.cy = 0
	}
	if E.cy >= len(E.rows) {
		E.cy = len(E.rows) - 1
	}
	if E.cx < 0 {
		E.cx = 0
	}
	if E.cx > len(E.rows[E.cy].s) {
		E.cx = len(E.rows[E.cy].s)
	}
	if E.rowoff >= len(E.rows) {
		E.rowoff = len(E.rows) - 1
	}

	g := gutterWidth()
	textCols := E.screenCols - g - 1
	if textCols < 1 {
		textCols = 1
	}
	if E.cy < E.rowoff {
		E.rowoff = E.cy
	}
	if E.cy >= E.rowoff+E.screenRows {
		E.rowoff = E.cy - E.screenRows + 1
	}
	if E.cx < E.coloff {
		E.coloff = E.cx
	}
	if E.cx >= E.coloff+textCols {
		E.coloff = E.cx - textCols + 1
	}
}

func refreshScreen() {
	updateAllSyntax(false)
	scroll()
	screenBuf.Reset()
	screenBuf.WriteString("\x1b[?25l\x1b[H")
	drawRows(&screenBuf)
	drawStatusBar(&screenBuf)
	drawMessageBar(&screenBuf)
	drawContextMenu(&screenBuf)

	// Save state for differential rendering
	if cap(E.lastRows) < E.screenRows {
		E.lastRows = make([]*row, E.screenRows)
	} else {
		E.lastRows = E.lastRows[:E.screenRows]
	}
	for y := 0; y < E.screenRows; y++ {
		fr := y + E.rowoff
		if fr < len(E.rows) {
			E.lastRows[y] = E.rows[fr]
		} else {
			E.lastRows[y] = nil
		}
	}
	E.lastRowoff = E.rowoff
	E.lastColoff = E.coloff

	g := gutterWidth()
	gcols := 0
	if g > 0 {
		gcols = g + 1
	}
	curRow := (E.cy - E.rowoff) + 1
	if curRow < 1 {
		curRow = 1
	}
	curCol := 1 + g + 1
	if E.cy >= 0 && E.cy < len(E.rows) {
		line := E.rows[E.cy].s
		start := utf8SnapBoundary(line, E.coloff)
		if start > len(line) {
			start = len(line)
		}
		end := E.cx
		if end > len(line) {
			end = len(line)
		}
		if end < start {
			end = start
		}
		curCol += displayWidthBytes(line[start:end], gcols)
	}
	if curCol < 1 {
		curCol = 1
	}
	if len(E.statusmsg) > 0 && E.statusmsg[0] == ':' {
		curRow = E.screenRows + 2
		curCol = len(E.statusmsg) + 1
	}
	writeCursorPos(&screenBuf, curRow, curCol)
	screenBuf.WriteString("\x1b[?25h")
	_, _ = os.Stdout.Write(screenBuf.Bytes())
}

func gutterWidth() int {
	if E.filename == "" && len(E.rows) == 0 {
		return 0
	}
	n := max(1, len(E.rows))
	w := 1
	for n >= 10 {
		n /= 10
		w++
	}
	return w
}

func byteIndexFromDisplayCol(s []byte, target int, colStart int) int {
	if target <= colStart {
		return 0
	}
	i := 0
	col := colStart
	for i < len(s) {
		r, n := utf8.DecodeRune(s[i:])
		if n <= 0 {
			n = 1
		}
		w := runeDisplayWidth(r)
		if r == '\t' {
			tabW := 8 - (col % 8)
			if tabW == 0 {
				tabW = 8
			}
			w = tabW
		}
		if col+w > target {
			break
		}
		if w < 0 {
			w = 1
		}
		col += w
		i += n
	}
	return i
}

func displayWidthBytes(s []byte, startCol int) int {
	col := startCol
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRune(s[i:])
		if n <= 0 {
			n = 1
		}
		w := runeDisplayWidth(r)
		if r == '\t' {
			if col%8 == 0 {
				w = 8
			} else {
				w = 8 - (col % 8)
			}
		}
		if w < 0 {
			w = 1
		}
		col += w
		i += n
	}
	return col - startCol
}
