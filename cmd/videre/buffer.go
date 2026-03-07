package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func setStatus(format string, args ...any) {
	E.statusmsg = fmt.Sprintf(format, args...)
	E.statusTime = time.Now()
}

func ioErrText(err error) string {
	if err == nil {
		return ""
	}
	var pe *os.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}

func validateFilename(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	if filepath.IsAbs(name) && len(name) > 4096 {
		return false
	}
	const dangerous = "<>\"|&;$`'()[]{}*?"
	for i := 0; i < len(name); i++ {
		if name[i] < 0x20 || name[i] == 0x7f {
			return false
		}
		if strings.ContainsRune(dangerous, rune(name[i])) {
			return false
		}
	}
	if st, err := os.Stat(name); err == nil {
		if !st.Mode().IsRegular() {
			return false
		}
		if st.Size() > 100*1024*1024 {
			return false
		}
	}
	return true
}

func normalizeFilename(name string) (string, error) {
	if name == "~" || strings.HasPrefix(name, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if name == "~" {
			return home, nil
		}
		return filepath.Join(home, name[2:]), nil
	}
	return name, nil
}

func insertRow(at int, s []byte) {
	if at < 0 || at > len(E.rows) {
		return
	}
	r := &row{idx: at, s: append([]byte(nil), s...), needsHighlight: true}
	E.rows = append(E.rows, nil)
	copy(E.rows[at+1:], E.rows[at:])
	E.rows[at] = r
	for i := at; i < len(E.rows); i++ {
		if E.rows[i].idx != i {
			E.rows[i] = duplicateRow(E.rows[i])
			E.rows[i].idx = i
		}
	}
	updateSyntax(E.rows[at], false)
	E.dirty = true
}

func duplicateRow(r *row) *row {
	if r == nil {
		return nil
	}
	return &row{
		idx:            r.idx,
		s:              append([]byte(nil), r.s...),
		hl:             append([]uint8(nil), r.hl...),
		open:           r.open,
		needsHighlight: r.needsHighlight,
		hlState:        r.hlState,
	}
}

func delRow(at int) {
	if at < 0 || at >= len(E.rows) {
		return
	}
	E.rows = append(E.rows[:at], E.rows[at+1:]...)
	for i := at; i < len(E.rows); i++ {
		if E.rows[i].idx != i {
			E.rows[i] = duplicateRow(E.rows[i])
			E.rows[i].idx = i
		}
	}
	E.dirty = true
}

func rowInsertChar(r *row, at int, c byte) {
	if at < 0 || at > len(r.s) {
		at = len(r.s)
	}
	r.s = append(r.s, 0)
	copy(r.s[at+1:], r.s[at:])
	r.s[at] = c
	updateSyntax(r, false)
	E.dirty = true
}

func rowDelChar(r *row, at int) {
	if at < 0 || at >= len(r.s) {
		return
	}
	copy(r.s[at:], r.s[at+1:])
	r.s = r.s[:len(r.s)-1]
	updateSyntax(r, false)
	E.dirty = true
}

func rowAppendString(r *row, s []byte) {
	r.s = append(r.s, s...)
	updateSyntax(r, false)
	E.dirty = true
}

func cloneRows(src []*row) []*row {
	out := make([]*row, len(src))
	copy(out, src)
	return out
}

func saveUndo() {
	s := undoState{rows: cloneRows(E.rows), cx: E.cx, cy: E.cy}
	E.undo = append(E.undo, s)
	E.redo = nil
}

func applyState(s undoState) {
	E.rows = cloneRows(s.rows)
	E.cx = s.cx
	E.cy = s.cy
	E.dirty = true
	updateAllSyntax(true)
}

func doUndo() {
	if len(E.undo) == 0 {
		return
	}
	E.redo = append(E.redo, undoState{rows: cloneRows(E.rows), cx: E.cx, cy: E.cy})
	last := E.undo[len(E.undo)-1]
	E.undo = E.undo[:len(E.undo)-1]
	applyState(last)
}

func doRedo() {
	if len(E.redo) == 0 {
		return
	}
	E.undo = append(E.undo, undoState{rows: cloneRows(E.rows), cx: E.cx, cy: E.cy})
	last := E.redo[len(E.redo)-1]
	E.redo = E.redo[:len(E.redo)-1]
	applyState(last)
}

func openFile(name string) bool {
	name, err := normalizeFilename(name)
	if err != nil {
		setStatus("Can't resolve path: %s", ioErrText(err))
		return false
	}
	if !validateFilename(name) {
		setStatus("Invalid filename or path")
		return false
	}
	f, err := os.Open(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			E.rows = nil
			E.filename = name
			E.dirty = false
			selectSyntax()
			updateGitStatus()
			updateAllSyntax(true)
			setStatus("\"%s\" [New File]", E.filename)
			return true
		}
		setStatus("Can't open file: %s", ioErrText(err))
		return false
	}
	defer f.Close()
	E.rows = nil
	r := bufio.NewReader(f)
	for {
		line, rerr := r.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			insertRow(len(E.rows), line)
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			setStatus("Read error: %s", ioErrText(rerr))
			break
		}
	}
	E.filename = name
	E.dirty = false
	selectSyntax()
	updateGitStatus()
	updateAllSyntax(true)
	return true
}

func rowsToString() []byte {
	var b bytes.Buffer
	for i := range E.rows {
		b.Write(E.rows[i].s)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func saveFile() {
	if E.filename == "" {
		name := prompt("Save as: %s", nil)
		if name == "" {
			setStatus("Save aborted")
			return
		}
		normalized, err := normalizeFilename(name)
		if err != nil {
			setStatus("Can't resolve path: %s", ioErrText(err))
			return
		}
		E.filename = normalized
		selectSyntax()
	}
	if !validateFilename(E.filename) {
		setStatus("Invalid filename for saving")
		return
	}
	data := rowsToString()
	if err := os.WriteFile(E.filename, data, 0o644); err != nil {
		setStatus("Can't save! I/O error: %s", ioErrText(err))
		return
	}
	E.dirty = false
	updateGitStatus()
	setStatus("\"%s\" %dL, %dC written", E.filename, len(E.rows), len(data))
}

func updateGitStatus() {
	E.gitStatus = ""
	if E.filename == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "status", "--porcelain", "-b").Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "## ") {
		return
	}
	branch := strings.TrimPrefix(lines[0], "## ")
	if i := strings.Index(branch, "..."); i >= 0 {
		branch = branch[:i]
	}
	if len(lines) > 1 {
		branch += "*"
	}
	if len(branch) > 32 {
		branch = branch[:32]
	}
	E.gitStatus = branch
}

func insertChar(c byte) {
	saveUndo()
	if E.cy == len(E.rows) {
		insertRow(len(E.rows), nil)
	}
	E.rows[E.cy] = duplicateRow(E.rows[E.cy])
	rowInsertChar(E.rows[E.cy], E.cx, c)
	E.rows[E.cy].needsHighlight = true
	E.cx = utf8NextBoundary(E.rows[E.cy].s, E.cx)
	E.preferred = E.cx
}

func insertNewline() {
	saveUndo()
	if E.cx == 0 {
		insertRow(E.cy, nil)
	} else {
		oldR := E.rows[E.cy]
		insertRow(E.cy+1, oldR.s[E.cx:])
		E.rows[E.cy] = duplicateRow(oldR)
		E.rows[E.cy].s = append([]byte(nil), oldR.s[:E.cx]...)
		updateSyntax(E.rows[E.cy], false)
	}
	E.cy++
	E.cx = 0
	E.preferred = 0
}

func delChar() {
	if E.cy == len(E.rows) || (E.cx == 0 && E.cy == 0) {
		return
	}
	saveUndo()
	if E.cx > 0 {
		E.rows[E.cy] = duplicateRow(E.rows[E.cy])
		r := E.rows[E.cy]
		prev := utf8PrevBoundary(r.s, E.cx)
		rowDelChar(r, prev)
		r.needsHighlight = true
		E.cx = prev
	} else {
		E.rows[E.cy-1] = duplicateRow(E.rows[E.cy-1])
		E.cx = len(E.rows[E.cy-1].s)
		rowAppendString(E.rows[E.cy-1], E.rows[E.cy].s)
		E.rows[E.cy-1].needsHighlight = true
		delRow(E.cy)
		E.cy--
	}
	E.preferred = E.cx
}
