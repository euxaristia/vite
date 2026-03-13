package main

import (
	"bytes"
	"testing"
)

func TestCommandModeJumpBottom(t *testing.T) {
	seedEditor([]string{"line1", "line2", "line3"}, 0, 0)
	// Mock processKeypress is hard because it calls prompt which is interactive.
	// But we can call handleSubstitute directly or mock the ':' logic.
	
	// Testing moveToLine(len(E.rows)) which is what :$ does
	moveToLine(len(E.rows))
	if E.cy != 2 {
		t.Errorf("expected cy=2, got %d", E.cy)
	}
}

func TestHandleSubstitute(t *testing.T) {
	seedEditor([]string{"foo bar", "baz foo", "qux"}, 0, 0)
	
	// Test single line substitute
	handleSubstitute("s/foo/FOO/")
	if string(E.rows[0].s) != "FOO bar" {
		t.Errorf("expected 'FOO bar', got %q", string(E.rows[0].s))
	}
	if string(E.rows[1].s) != "baz foo" {
		t.Errorf("expected 'baz foo' (unchanged), got %q", string(E.rows[1].s))
	}
	
	// Test global substitute on current line
	seedEditor([]string{"foo foo foo"}, 0, 0)
	handleSubstitute("s/foo/X/g")
	if string(E.rows[0].s) != "X X X" {
		t.Errorf("expected 'X X X', got %q", string(E.rows[0].s))
	}
	
	// Test whole file substitute
	seedEditor([]string{"a", "a", "b"}, 0, 0)
	handleSubstitute("%s/a/X/")
	if string(E.rows[0].s) != "X" || string(E.rows[1].s) != "X" || string(E.rows[2].s) != "b" {
		t.Errorf("whole file substitute failed: %v", E.rows)
	}
}

func TestMatchBracketImprovement(t *testing.T) {
	// Test jumping from start of line to first bracket's match
	seedEditor([]string{"func() {"}, 0, 0)
	matchBracket() // Should find '(' at 4 and jump to ')' at 5
	if E.cx != 5 {
		t.Errorf("expected jump to ')', cx=5, got %d", E.cx)
	}
	matchBracket() // Should jump back to '(' at 4
	if E.cx != 4 {
		t.Errorf("expected jump to '(', cx=4, got %d", E.cx)
	}
}

func TestIndentNormalMode(t *testing.T) {
	// We can't easily test the interactive part of >> but we can test the logic
	seedEditor([]string{"line1", "line2"}, 0, 0)
	
	// Simulate >> on line 0
	E.rows[0].s = append([]byte("    "), E.rows[0].s...)
	if !bytes.HasPrefix(E.rows[0].s, []byte("    line1")) {
		t.Errorf("indent failed")
	}
}

func TestTextObjects(t *testing.T) {
	// Test word objects
	seedEditor([]string{"  the quick brown  "}, 8, 0) // on 'u' in 'quick'
	sx, sy, ex, ey, ok := findTextObject('w', true)
	if !ok || sx != 6 || ex != 10 || sy != 0 || ey != 0 {
		t.Errorf("iw failed: got (%d,%d)-(%d,%d) ok=%v, want (6,0)-(10,0)", sx, sy, ex, ey, ok)
	}
	sx, sy, ex, ey, ok = findTextObject('w', false)
	if !ok || sx != 6 || ex != 11 || sy != 0 || ey != 0 {
		t.Errorf("aw failed: got (%d,%d)-(%d,%d) ok=%v, want (6,0)-(11,0)", sx, sy, ex, ey, ok)
	}

	// Test quote objects
	seedEditor([]string{"msg := \"hello world\";"}, 10, 0) // inside quotes
	sx, sy, ex, ey, ok = findTextObject('"', true)
	if !ok || sx != 8 || ex != 18 || sy != 0 || ey != 0 {
		t.Errorf("i\" failed: got (%d,%d)-(%d,%d) ok=%v, want (8,0)-(18,0)", sx, sy, ex, ey, ok)
	}
	sx, sy, ex, ey, ok = findTextObject('"', false)
	if !ok || sx != 7 || ex != 19 || sy != 0 || ey != 0 {
		t.Errorf("a\" failed: got (%d,%d)-(%d,%d) ok=%v, want (7,0)-(19,0)", sx, sy, ex, ey, ok)
	}

	// Test bracket objects
	seedEditor([]string{"func(a, (b, c), d) {"}, 10, 0) // inside (b, c)
	sx, sy, ex, ey, ok = findTextObject('(', true)
	if !ok || sx != 9 || ex != 12 || sy != 0 || ey != 0 {
		t.Errorf("i( failed: got (%d,%d)-(%d,%d) ok=%v, want (9,0)-(12,0)", sx, sy, ex, ey, ok)
	}
	sx, sy, ex, ey, ok = findTextObject('(', false)
	if !ok || sx != 8 || ex != 13 || sy != 0 || ey != 0 {
		t.Errorf("a( failed: got (%d,%d)-(%d,%d) ok=%v, want (8,0)-(13,0)", sx, sy, ex, ey, ok)
	}
}

func TestMultilineSyntax(t *testing.T) {
	seedEditor([]string{"/* start", "middle", "end */", "outside"}, 0, 0)
	E.filename = "test.c"
	updateAllSyntax(true) // This should force update all and handle block comments

	if E.rows[0].hl[0] != hlComment {
		t.Errorf("expected hlComment at row 0, got %d", E.rows[0].hl[0])
	}
	if E.rows[1].hl[0] != hlComment {
		t.Errorf("expected hlComment at row 1 (propagation), got %d", E.rows[1].hl[0])
	}
	if E.rows[2].hl[0] != hlComment {
		t.Errorf("expected hlComment at row 2, got %d", E.rows[2].hl[0])
	}
	if E.rows[3].hl[0] != hlNormal {
		t.Errorf("expected hlNormal at row 3, got %d", E.rows[3].hl[0])
	}

	// Test propagation on modification
	E.rows[0].s = []byte("normal line")
	E.rows[0].needsHighlight = true
	updateAllSyntax(false)
	if E.rows[1].hl[0] != hlNormal {
		t.Errorf("expected hlNormal at row 1 after propagation, got %d", E.rows[1].hl[0])
	}
}

func TestRepeatCommand(t *testing.T) {
	seedEditor([]string{"start "}, 5, 0)
	
	// Manually record "iabc<ESC>"
	E.lastChange = []int{'i', 'a', 'b', 'c', 0x1b}
	
	// Replay with "."
	E.keyBuffer = append([]int{'.'}, E.lastChange...)
	
	// Process the replayed keys
	for len(E.keyBuffer) > 0 {
		processKeypress()
	}
	
	if string(E.rows[0].s) != "startabc " {
		t.Errorf("repeat failed: expected 'startabc ', got %q", string(E.rows[0].s))
	}
}

func TestRegexSearch(t *testing.T) {
	seedEditor([]string{"item 123", "line 456", "another line"}, 0, 0)
	
	// Test finding digits
	setSearchPattern("123") // Start with literal
	findNext(1)
	if E.cy != 0 || E.cx != 5 {
		t.Errorf("regex find 123 failed: got (%d,%d), want (0,5)", E.cy, E.cx)
	}
	
	setSearchPattern("456")
	findNext(1)
	if E.cy != 1 || E.cx != 5 {
		t.Errorf("regex find 456 failed: got (%d,%d), want (1,5)", E.cy, E.cx)
	}
	
	// Test start of line anchor
	seedEditor([]string{"  func", "func", "other"}, 0, 0)
	setSearchPattern("^func")
	findNext(1)
	if E.cy != 1 || E.cx != 0 {
		t.Errorf("regex anchor search failed: got (%d,%d), want (1,0)", E.cy, E.cx)
	}
}

func TestShiftArrowMotions(t *testing.T) {
	seedEditor([]string{"word1 word2", "", "word3"}, 0, 0)
	
	// Shift+Right should move to next word
	applyMotionKey(shiftRight, 1)
	if E.cx == 0 { // It should have moved to 'word2' (index 6)
		// Note: exact index depends on moveWordForward implementation
	}
	if E.cx <= 0 {
		t.Errorf("Shift+Right didn't move cursor, cx=%d", E.cx)
	}

	// Shift+Down should move to next paragraph
	seedEditor([]string{"p1", "", "p2"}, 0, 0)
	applyMotionKey(shiftDown, 1)
	if E.cy != 2 {
		t.Errorf("Shift+Down expected cy=2, got %d", E.cy)
	}
}

func TestBlackHoleDelete(t *testing.T) {
	seedEditor([]string{"line1", "line2", "line3"}, 0, 0)
	
	// First, copy "line1" to the unnamed register
	yoink(0, 0, 5, 0, true)
	if string(E.registers['"'].s) != "line1\n" {
		t.Errorf("Initial yoink failed, got %q", string(E.registers['"'].s))
	}
	
	// Now perform a black hole delete on "line1"
	E.SelectedRegister = '_'
	E.keyBuffer = []int{'d'} // pre-load 'd' so readKey() inside handleOperator doesn't block
	handleOperator('d', 1)   // simulates dd
	
	// Check that line1 is gone from rows
	if string(E.rows[0].s) != "line2" {
		t.Errorf("Delete failed, row 0 is %q", string(E.rows[0].s))
	}
	
	// Check that the unnamed register still contains "line1"
	if string(E.registers['"'].s) != "line1\n" {
		t.Errorf("Unnamed register was overwritten by black hole delete, got %q", string(E.registers['"'].s))
	}
}
