package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

var inputBuffer []byte

func unreadByte(b byte) {
	inputBuffer = append(inputBuffer, b)
}

func readByte(fd int) (byte, error) {
	if len(inputBuffer) > 0 {
		b := inputBuffer[0]
		inputBuffer = inputBuffer[1:]
		return b, nil
	}
	var b [1]byte
	for {
		if atomic.SwapInt32(&resizePending, 0) != 0 {
			return 0, syscall.EINTR
		}
		n, err := syscall.Read(fd, b[:])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return 0, syscall.EINTR
			}
			if errors.Is(err, syscall.EAGAIN) {
				continue
			}
			return 0, err
		}
		if n == 0 {
			if atomic.SwapInt32(&resizePending, 0) != 0 {
				return 0, syscall.EINTR
			}
			continue
		}
		return b[0], nil
	}
}

func readByteTimeout(fd int, maxPolls int) (byte, bool, error) {
	if len(inputBuffer) > 0 {
		b := inputBuffer[0]
		inputBuffer = inputBuffer[1:]
		return b, true, nil
	}
	var b [1]byte
	polls := 0
	for polls < maxPolls {
		if atomic.SwapInt32(&resizePending, 0) != 0 {
			return 0, false, syscall.EINTR
		}
		n, err := syscall.Read(fd, b[:])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return 0, false, syscall.EINTR
			}
			if errors.Is(err, syscall.EAGAIN) {
				continue
			}
			return 0, false, err
		}
		if n == 0 {
			if atomic.SwapInt32(&resizePending, 0) != 0 {
				return 0, false, syscall.EINTR
			}
			polls++
			continue
		}
		return b[0], true, nil
	}
	return 0, false, nil
}

func inputReady(fd int) bool {
	var rfds syscall.FdSet
	rfds.Bits[fd/64] |= 1 << (uint(fd) % 64)
	tv := syscall.Timeval{Sec: 0, Usec: 0}
	n, err := syscall.Select(fd+1, &rfds, nil, nil, &tv)
	return err == nil && n > 0
}

func parseSGRMouse(seq []byte) (mb, mx, my int, ok bool) {
	if len(seq) < 6 || seq[0] != '<' {
		return 0, 0, 0, false
	}
	end := len(seq) - 1
	if seq[end] != 'M' && seq[end] != 'm' {
		return 0, 0, 0, false
	}
	part := 0
	val := 0
	haveDigit := false
	for i := 1; i < end; i++ {
		c := seq[i]
		if c >= '0' && c <= '9' {
			haveDigit = true
			val = val*10 + int(c-'0')
			continue
		}
		if c != ';' || !haveDigit {
			return 0, 0, 0, false
		}
		switch part {
		case 0:
			mb = val
		case 1:
			mx = val
		default:
			return 0, 0, 0, false
		}
		part++
		val = 0
		haveDigit = false
	}
	if part != 2 || !haveDigit {
		return 0, 0, 0, false
	}
	my = val
	return mb, mx, my, true
}

func logDebug(format string, v ...interface{}) {
}

func readKey() int {
	if len(E.keyBuffer) > 0 {
		key := E.keyBuffer[0]
		E.keyBuffer = E.keyBuffer[1:]
		return key
	}
	fd := int(os.Stdin.Fd())
	first, err := readByte(fd)
	if err != nil {
		if errors.Is(err, syscall.EINTR) {
			return resizeEvent
		}
		die(err)
	}
	res := 0
	if first != 0x1b {
		res = int(first)
	} else if !inputReady(fd) {
		res = 0x1b
	} else {
		b, ok, err := readByteTimeout(fd, 1)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return resizeEvent
			}
			die(err)
		}
		if !ok {
			res = 0x1b
		} else if b == '[' {
			var seq [32]byte
			seqLen := 0
			for i := 0; i < 31; i++ {
				nb, has, rerr := readByteTimeout(fd, 1)
				if rerr != nil {
					if errors.Is(rerr, syscall.EINTR) {
						return resizeEvent
					}
					die(rerr)
				}
				if !has {
					break
				}
				seq[seqLen] = nb
				seqLen++
				if nb == '~' || nb == 'm' || nb == 'M' || (nb >= 'A' && nb <= 'Z') || (nb >= 'a' && nb <= 'z') {
					break
				}
			}
			if seqLen == 0 {
				res = 0x1b
			} else {
				seqb := seq[:seqLen]
				if len(seqb) == 4 && seqb[0] == '2' && seqb[1] == '0' && seqb[2] == '0' && seqb[3] == '~' {
					var paste bytes.Buffer
					for {
						ch, rerr := readByte(fd)
						if rerr != nil {
							if errors.Is(rerr, syscall.EINTR) {
								return resizeEvent
							}
							die(rerr)
						}
						if ch == 0x1b {
							nb, has, rerr := readByteTimeout(fd, 1)
							if rerr != nil {
								die(rerr)
							}
							if has && nb == '[' {
								var endSeq [8]byte
								endLen := 0
								for i := 0; i < 5; i++ {
									b2, has2, _ := readByteTimeout(fd, 1)
									if !has2 {
										break
									}
									endSeq[endLen] = b2
									endLen++
									if b2 == '~' {
										break
									}
								}
								endb := endSeq[:endLen]
								if len(endb) == 4 && endb[0] == '2' && endb[1] == '0' && endb[2] == '1' && endb[3] == '~' {
									E.pasteBuffer = paste.Bytes()
									res = pasteEvent
									goto end
								}
								paste.WriteByte(0x1b)
								paste.WriteByte('[')
								paste.Write(endb)
								continue
							}
							paste.WriteByte(0x1b)
							if has {
								paste.WriteByte(nb)
							}
							continue
						}
						paste.WriteByte(ch)
					}
				}
				if len(seqb) >= 2 && seqb[0] == '<' && (seqb[len(seqb)-1] == 'm' || seqb[len(seqb)-1] == 'M') {
					mb, mx, my, ok := parseSGRMouse(seqb)
					if ok {
						E.mouseB = mb
						E.mouseX = mx
						E.mouseY = my
						if seqb[len(seqb)-1] == 'm' {
							E.mouseB |= 0x80
						}
						res = mouseEvent
						goto end
					}
				}
				if len(seqb) >= 2 && seqb[len(seqb)-1] == '~' && seqb[0] >= '0' && seqb[0] <= '9' {
					switch seqb[0] {
					case '1', '7':
						res = homeKey
						goto end
					case '3':
						res = delKey
						goto end
					case '4', '8':
						res = endKey
						goto end
					case '5':
						res = pageUp
						goto end
					case '6':
						res = pageDown
						goto end
					}
				}
				if len(seqb) == 5 && seqb[0] == '1' && seqb[1] == ';' && seqb[2] == '2' {
					switch seqb[4] {
					case 'A':
						res = shiftUp
						goto end
					case 'B':
						res = shiftDown
						goto end
					case 'C':
						res = shiftRight
						goto end
					case 'D':
						res = shiftLeft
						goto end
					}
				}
				if len(seqb) == 5 && seqb[2] == ';' && seqb[3] == '6' && seqb[4] == 'u' && (string(seqb[:2]) == "99" || string(seqb[:2]) == "67") {
					res = ctrlShiftC
					goto end
				}
				switch seqb[len(seqb)-1] {
				case 'A':
					res = arrowUp
				case 'B':
					res = arrowDown
				case 'C':
					res = arrowRight
				case 'D':
					res = arrowLeft
				case 'H':
					res = homeKey
				case 'F':
					res = endKey
				default:
					res = 0x1b
				}
			}
		} else if b == 'O' {
			nb, has, rerr := readByteTimeout(fd, 1)
			if rerr != nil {
				die(rerr)
			}
			if !has {
				res = 0x1b
			} else {
				switch nb {
				case 'H':
					res = homeKey
				case 'F':
					res = endKey
				default:
					unreadByte(b)
					unreadByte(nb)
					res = 0x1b
				}
			}
		} else {
			unreadByte(b)
			res = 0x1b
		}
	}

end:
	if E.recordingChange && res != 0 && res != resizeEvent {
		E.currentChange = append(E.currentChange, res)
	}
	return res
}

func enableRawMode() {
	fd := int(os.Stdin.Fd())
	t, err := ioctlGetTermios(fd, syscall.TCGETS)
	if err != nil {
		return
	}
	E.termOrig = *t
	raw := *t
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := ioctlSetTermios(fd, syscall.TCSETS, &raw); err != nil {
		return
	}
	E.raw = true
	fmt.Print(termEnterSeq)
}

func disableRawMode() {
	if !E.raw {
		return
	}
	_ = ioctlSetTermios(int(os.Stdin.Fd()), syscall.TCSETS, &E.termOrig)
	fmt.Print(termLeaveSeq)
	E.raw = false
}

func handleMouse() bool {
	b := E.mouseB
	x := E.mouseX
	y := E.mouseY

	if E.menuOpen {
		prevSelected := E.menuSelected
		menuW := contextMenuW + 2
		menuH := len(menuItems) + 2
		mx, my := E.menuX, E.menuY
		if mx+menuW > E.screenCols {
			mx = E.screenCols - menuW
		}
		if my+menuH > E.screenRows {
			my = E.screenRows - menuH
		}
		if mx < 1 {
			mx = 1
		}
		if my < 1 {
			my = 1
		}
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
		if b&0x80 != 0 || b&mouseDrag != 0 {
			return E.menuSelected != prevSelected
		}
		E.menuOpen = false
		return true
	}

	if b&0x40 != 0 {
		if (b&0x3) == 0 || b == mouseWheelUp {
			for i := 0; i < 3; i++ {
				if E.rowoff > 0 {
					E.rowoff--
					if E.cy >= E.rowoff+E.screenRows {
						E.cy = E.rowoff + E.screenRows - 1
					}
				}
			}
		} else if (b&0x3) == 1 || b == mouseWheelDown {
			for i := 0; i < 3; i++ {
				if E.rowoff+E.screenRows < len(E.rows) {
					E.rowoff++
					if E.cy < E.rowoff {
						E.cy = E.rowoff
					}
				}
			}
		}
		if E.cy >= 0 && E.cy < len(E.rows) {
			limit := len(E.rows[E.cy].s)
			if E.mode != modeInsert && limit > 0 {
				limit = utf8PrevBoundary(E.rows[E.cy].s, limit)
			}
			if E.preferred > limit {
				E.cx = limit
			} else {
				E.cx = E.preferred
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

	if b&0x80 != 0 {
		E.isDragging = false
		return false
	}
	if b != mouseLeft && b != (mouseLeft|mouseDrag) {
		E.isDragging = false
		return false
	}
	prevCX, prevCY := E.cx, E.cy

	applyMousePosition := func() bool {
		if len(E.rows) == 0 {
			return false
		}
		fr := y - 1 + E.rowoff
		if fr < 0 || fr >= len(E.rows) {
			return false
		}
		E.cy = fr
		g := gutterWidth()
		gcols := 0
		if g > 0 {
			gcols = g + 1
		}
		textX := x - gcols
		target := 0
		if textX > 1 {
			target = textX - 1
		}
		target += gcols
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
