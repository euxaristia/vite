package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

func readKey() int {
	if len(E.keyBuffer) > 0 {
		c := E.keyBuffer[0]
		E.keyBuffer = E.keyBuffer[1:]
		return c
	}
	for {
		ev := E.Screen.PollEvent()
		if ev == nil {
			return -1
		}
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyRune:
				return int(ev.Rune())
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				return backspace
			case tcell.KeyEnter:
				return '\r'
			case tcell.KeyEsc:
				return '\x1b'
			case tcell.KeyLeft:
				return arrowLeft
			case tcell.KeyRight:
				return arrowRight
			case tcell.KeyUp:
				return arrowUp
			case tcell.KeyDown:
				return arrowDown
			case tcell.KeyDelete:
				return delKey
			case tcell.KeyHome:
				return homeKey
			case tcell.KeyEnd:
				return endKey
			case tcell.KeyPgUp:
				return pageUp
			case tcell.KeyPgDn:
				return pageDown
			case tcell.KeyCtrlC:
				return 3
			case tcell.KeyCtrlD:
				return 4
			case tcell.KeyCtrlU:
				return 21
			case tcell.KeyCtrlR:
				return 18
			case tcell.KeyCtrlA:
				return 1
			case tcell.KeyCtrlX:
				return 24
			}
		case *tcell.EventMouse:
			x, y := ev.Position()
			E.mouseX, E.mouseY = x, y
			button := ev.Buttons()
			if button&tcell.ButtonPrimary != 0 {
				E.mouseB = mouseLeft
			} else if button&tcell.ButtonSecondary != 0 {
				E.mouseB = mouseRight
			} else if button&tcell.WheelUp != 0 {
				E.mouseB = mouseWheelUp
			} else if button&tcell.WheelDown != 0 {
				E.mouseB = mouseWheelDown
			} else if button == tcell.ButtonNone {
				E.mouseB = -1 
			}
			
			if button&tcell.ButtonPrimary != 0 && E.isDragging {
				E.mouseB = mouseLeft | mouseDrag
			}
			
			return mouseEvent
		case *tcell.EventResize:
			updateWindowSize()
			return resizeEvent
		case *tcell.EventPaste:
			return pasteEvent
		}
	}
}

func handleMouse() bool {
	b := E.mouseB
	x := E.mouseX
	y := E.mouseY

	if E.menuOpen {
		menuW := contextMenuW + 2
		menuH := len(menuItems) + 2
		mx, my := E.menuX, E.menuY
		if mx+menuW > E.screenCols {
			mx = E.screenCols - menuW
		}
		if my+menuH > E.screenRows {
			my = E.screenRows - menuH
		}
		if mx < 0 { mx = 0 }
		if my < 0 { my = 0 }
		
		if x >= mx && x < mx+menuW && y >= my && y < my+menuH {
			E.menuSelected = y - my - 1
			if E.menuSelected < 0 || E.menuSelected >= len(menuItems) {
				E.menuSelected = -1
			}
		} else {
			E.menuSelected = -1
		}
		if b == mouseLeft {
			if E.menuSelected >= 0 {
				executeMenuAction(E.menuSelected)
			}
			E.menuOpen = false
			return true
		}
		if b == mouseRight {
			E.menuX, E.menuY = x, y
			E.menuSelected = -1
			return true
		}
		E.menuOpen = false
		return true
	}

	if b == mouseWheelUp {
		for i := 0; i < 3; i++ {
			if E.rowoff > 0 {
				E.rowoff--
				if E.cy >= E.rowoff+E.screenRows {
					E.cy = E.rowoff + E.screenRows - 1
				}
			}
		}
		return true
	} else if b == mouseWheelDown {
		for i := 0; i < 3; i++ {
			if E.rowoff+E.screenRows < len(E.rows) {
				E.rowoff++
				if E.cy < E.rowoff {
					E.cy = E.rowoff
				}
			}
		}
		return true
	}

	if b == mouseRight {
		E.menuOpen = true
		E.menuX = x
		E.menuY = y
		E.menuSelected = -1
		return true
	}

	if b == -1 {
		E.isDragging = false
		return false
	}
	
	prevCX, prevCY := E.cx, E.cy

	applyMousePosition := func() bool {
		if len(E.rows) == 0 {
			return false
		}
		fr := y + E.rowoff
		if fr < 0 || fr >= len(E.rows) {
			return false
		}
		E.cy = fr
		g := gutterWidth()
		gcols := 0
		if g > 0 {
			gcols = g + 1
		}
		target := x
		start := utf8SnapBoundary(E.rows[E.cy].s, E.coloff)
		if start > len(E.rows[E.cy].s) {
			start = len(E.rows[E.cy].s)
		}
		rel := byteIndexFromDisplayCol(E.rows[E.cy].s[start:], target, gcols)
		E.cx = start + rel
		if E.mode != modeInsert && len(E.rows[E.cy].s) > 0 && E.cx >= len(E.rows[E.cy].s) {
			E.cx = len(E.rows[E.cy].s) - 1
		}
		E.preferred = E.cx
		return true
	}

	if b == (mouseLeft | mouseDrag) {
		if !E.isDragging {
			return false
		}
		if !applyMousePosition() {
			return false
		}
		if E.mode == modeNormal {
			E.mode = modeVisual
		}
		return true
	}

	if b == mouseLeft {
		if !applyMousePosition() {
			return false
		}
		now := time.Now()
		doubleClick := x == E.lastClickX && y == E.lastClickY &&
			!E.lastClickTime.IsZero() && now.Sub(E.lastClickTime) < 500*time.Millisecond

		if doubleClick {
			selectWord()
			E.isDragging = false
			if E.mode != modeVisual && E.mode != modeVisualLine {
				E.mode = modeVisual
			}
		} else {
			E.isDragging = true
			E.selSX = E.cx
			E.selSY = E.cy
			if E.mode == modeVisual || E.mode == modeVisualLine {
				E.mode = modeNormal
				E.selSX, E.selSY = -1, -1
			}
		}
		E.lastClickX = x
		E.lastClickY = y
		E.lastClickTime = now
		return true
	}
	return E.cx != prevCX || E.cy != prevCY
}

func enableRawMode() {}
func disableRawMode() {}
