package main

import (
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

var syntaxStyleLUT = map[uint8]tcell.Style{
	hlNormal:      tcell.StyleDefault.Foreground(tcell.ColorWhite),
	hlComment:     tcell.StyleDefault.Foreground(tcell.ColorGreen),
	hlKeyword1:    tcell.StyleDefault.Foreground(tcell.ColorYellow),
	hlKeyword2:    tcell.StyleDefault.Foreground(tcell.ColorAqua),
	hlString:      tcell.StyleDefault.Foreground(tcell.ColorDarkMagenta),
	hlNumber:      tcell.StyleDefault.Foreground(tcell.ColorRed),
	hlMatch:       tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy),
	hlMatchCursor: tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow),
}

func runeDisplayWidth(r rune) int {
	if r < 0x20 || (r >= 0x7F && r < 0xA0) {
		return 0
	}
	// Combining characters, zero-width joiners, variation selectors
	if (r >= 0x0300 && r <= 0x036F) || (r >= 0x1AB0 && r <= 0x1AFF) ||
		(r >= 0x1DC0 && r <= 0x1DFF) || (r >= 0x20D0 && r <= 0x20FF) ||
		(r >= 0xFE20 && r <= 0xFE2F) || r == 0x200D ||
		(r >= 0xFE00 && r <= 0xFE0F) {
		return 0
	}
	// East Asian Fullwidth/Wide characters
	if (r >= 0x1100 && r <= 0x115F) || (r >= 0x2329 && r <= 0x232A) ||
		(r >= 0x2E80 && r <= 0xA4CF) || (r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0xFE10 && r <= 0xFE19) ||
		(r >= 0xFE30 && r <= 0xFE6F) || (r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) {
		return 2
	}
	// Emoji ranges (approximate)
	if (r >= 0x1F300 && r <= 0x1F64F) || (r >= 0x1F680 && r <= 0x1F6FF) ||
		(r >= 0x1F900 && r <= 0x1F9FF) || (r >= 0x2600 && r <= 0x27BF) {
		return 2
	}
	return 1
}

func drawRows() {
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

	for y := 0; y < E.screenRows; y++ {
		fr := y + E.rowoff
		if fr >= len(E.rows) {
			if len(E.rows) == 0 && y >= E.screenRows/3 && y < E.screenRows/3+len(welcomeLines) {
				E.Screen.SetContent(0, y, '~', nil, tcell.StyleDefault.Foreground(tcell.ColorDimGray))
				msg := welcomeLines[y-E.screenRows/3]
				padding := (textCols - len(msg)) / 2
				for i, r := range msg {
					E.Screen.SetContent(gcols+padding+i, y, r, nil, tcell.StyleDefault)
				}
			} else {
				E.Screen.SetContent(0, y, '~', nil, tcell.StyleDefault.Foreground(tcell.ColorDimGray))
			}
			continue
		}

		// Draw gutter
		if g > 0 {
			lineNum := strconv.Itoa(fr + 1)
			for i := 0; i < g-len(lineNum); i++ {
				E.Screen.SetContent(i, y, ' ', nil, tcell.StyleDefault.Foreground(tcell.ColorDimGray))
			}
			for i, r := range lineNum {
				E.Screen.SetContent(g-len(lineNum)+i, y, r, nil, tcell.StyleDefault.Foreground(tcell.ColorDimGray))
			}
			E.Screen.SetContent(g, y, ' ', nil, tcell.StyleDefault)
		}

		rowData := E.rows[fr]
		updateSyntax(rowData, false)
		line := rowData.s
		start := utf8SnapBoundary(line, E.coloff)
		
		visible := line[start:]
		hl := rowData.hl[start:]
		rowInSelection := hasSelection && fr >= sy && fr <= ey
		
		col := 0
		for i := 0; i < len(visible) && col < textCols; {
			r, n := utf8.DecodeRune(visible[i:])
			if n == 0 { break }
			
			h := hl[i]
			style := syntaxStyleLUT[h]
			
			// Selection logic
			sel := false
			if rowInSelection {
				x := i + start
				if lineSelection {
					sel = true
				} else {
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
			if sel {
				style = style.Background(tcell.ColorDimGray)
			}

			if r == '\t' {
				tabW := 8 - ((gcols + col) % 8)
				for j := 0; j < tabW && col < textCols; j++ {
					E.Screen.SetContent(gcols+col, y, ' ', nil, style)
					col++
				}
			} else {
				E.Screen.SetContent(gcols+col, y, r, nil, style)
				col++
			}
			i += n
		}
	}
}

func drawStatusBar() {
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorDarkSlateGray)
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
			rx += runeDisplayWidth(r)
			i += n
		}
	}
	
	loc := fmt.Sprintf("%d,%d-%d", E.cy+1, E.cx+1, rx+1)
	right := fmt.Sprintf(" %-14s %s ", loc, pos)
	
	for i := 0; i < E.screenCols; i++ {
		E.Screen.SetContent(i, E.screenRows, ' ', nil, style)
	}
	
	for i, r := range left {
		if i >= E.screenCols-len(right) { break }
		E.Screen.SetContent(i, E.screenRows, r, nil, style)
	}
	
	for i, r := range right {
		E.Screen.SetContent(E.screenCols-len(right)+i, E.screenRows, r, nil, style)
	}
}

func drawMessageBar() {
	style := tcell.StyleDefault
	y := E.screenRows + 1
	for i := 0; i < E.screenCols; i++ {
		E.Screen.SetContent(i, y, ' ', nil, style)
	}

	if E.statusmsg != "" && time.Since(E.statusTime) < 5*time.Second {
		for i, r := range E.statusmsg {
			if i >= E.screenCols { break }
			E.Screen.SetContent(i, y, r, nil, style)
		}
		return
	}

	var modeStr string
	switch E.mode {
	case modeInsert:
		modeStr = "-- INSERT --"
	case modeVisual:
		modeStr = "-- VISUAL --"
	case modeVisualLine:
		modeStr = "-- VISUAL LINE --"
	}
	for i, r := range modeStr {
		E.Screen.SetContent(i, y, r, nil, style)
	}
}

func scroll() {
	if E.rowoff < 0 { E.rowoff = 0 }
	if E.coloff < 0 { E.coloff = 0 }
	if len(E.rows) == 0 {
		E.cx, E.cy, E.preferred = 0, 0, 0
		return
	}
	if E.cy < 0 { E.cy = 0 }
	if E.cy >= len(E.rows) { E.cy = len(E.rows) - 1 }
	if E.cx < 0 { E.cx = 0 }
	if E.cx > len(E.rows[E.cy].s) { E.cx = len(E.rows[E.cy].s) }

	g := gutterWidth()
	textCols := E.screenCols - g - 1
	if textCols < 1 { textCols = 1 }
	
	if E.cy < E.rowoff { E.rowoff = E.cy }
	if E.cy >= E.rowoff+E.screenRows { E.rowoff = E.cy - E.screenRows + 1 }
	if E.cx < E.coloff { E.coloff = E.cx }
	if E.cx >= E.coloff+textCols { E.coloff = E.cx - textCols + 1 }
}

func refreshScreen() {
	updateAllSyntax(false)
	scroll()
	E.Screen.Clear()
	
	drawRows()
	drawStatusBar()
	drawMessageBar()
	drawContextMenu()
	
	// Cursor positioning
	g := gutterWidth()
	gcols := 0
	if g > 0 { gcols = g + 1 }
	
	curY := E.cy - E.rowoff
	curX := gcols
	if E.cy >= 0 && E.cy < len(E.rows) {
		line := E.rows[E.cy].s
		start := utf8SnapBoundary(line, E.coloff)
		end := E.cx
		if end > len(line) { end = len(line) }
		if end < start { end = start }
		curX += displayWidthBytes(line[start:end], gcols)
	}

	if len(E.statusmsg) > 0 && E.statusmsg[0] == ':' {
		curY = E.screenRows + 1
		curX = len(E.statusmsg)
	}
	
	E.Screen.ShowCursor(curX, curY)
	E.Screen.Show()
}

func gutterWidth() int {
	if E.filename == "" && len(E.rows) == 0 { return 0 }
	n := max(1, len(E.rows))
	w := 1
	for n >= 10 {
		n /= 10
		w++
	}
	return w
}

func displayWidthBytes(s []byte, startCol int) int {
	col := startCol
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRune(s[i:])
		if n <= 0 { n = 1 }
		w := runeDisplayWidth(r)
		if r == '\t' {
			w = 8 - (col % 8)
		}
		col += w
		i += n
	}
	return col - startCol
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

func initContextMenuMetrics() {
	w := 1
	for _, item := range menuItems {
		if len(item) > w {
			w = len(item)
		}
	}
	contextMenuW = w
}

func drawContextMenu() {
	// Implementation for tcell menu if needed
}

func updateWindowSize() {
	w, h := E.Screen.Size()
	E.screenCols = w
	E.screenRows = h - 2
}

func byteIndexFromDisplayCol(s []byte, target int, colStart int) int {
	if target <= colStart {
		return 0
	}
	i := 0
	col := colStart
	for i < len(s) {
		r, n := utf8.DecodeRune(s[i:])
		if n <= 0 { n = 1 }
		w := runeDisplayWidth(r)
		if r == '\t' {
			tabW := 8 - (col % 8)
			if tabW == 0 { tabW = 8 }
			w = tabW
		}
		if col+w > target { break }
		col += w
		i += n
	}
	return i
}
