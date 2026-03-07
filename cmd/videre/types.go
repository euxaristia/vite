package main

import (
	"bytes"
	"regexp"
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/sys/unix"
)

const (
	backspace = 1000 + iota
	arrowLeft
	arrowRight
	arrowUp
	arrowDown
	delKey
	homeKey
	endKey
	pageUp
	pageDown
	shiftUp
	shiftDown
	shiftRight
	shiftLeft
	ctrlShiftC
	mouseEvent
	pasteEvent
	resizeEvent
)

const (
	modeNormal = iota
	modeInsert
	modeVisual
	modeVisualLine
)

const (
	hlNormal uint8 = iota
	hlComment
	hlKeyword1
	hlKeyword2
	hlString
	hlNumber
	hlMatch
	hlMatchCursor
	hlVisual
)

const (
	mouseLeft      = 0
	mouseRight     = 2
	mouseWheelUp   = 64
	mouseWheelDown = 65
	mouseDrag      = 32
)

type row struct {
	idx            int
	s              []byte
	hl             []uint8
	open           bool
	needsHighlight bool
	hlState        int
}

type reg struct {
	s      []byte
	isLine bool
}

type undoState struct {
	rows []*row
	cx   int
	cy   int
}

type keyword struct {
	lit  []byte
	kind uint8
}

type editor struct {
	Screen            tcell.Screen
	cx, cy, preferred int
	rowoff, coloff    int
	screenRows        int
	screenCols        int
	rows              []*row
	dirty             bool
	filename          string
	gitStatus         string
	statusmsg         string
	statusTime        time.Time
	mode              int
	selSX, selSY      int
	searchPattern     string
	searchBytes       []byte
	searchRegexp      *regexp.Regexp
	lastSearchChar    byte
	lastSearchDir     int
	lastSearchTill    bool
	quitWarnRemaining int
	mouseX            int
	mouseY            int
	mouseB            int
	pasteBuffer       []byte
	menuOpen          bool
	menuX             int
	menuY             int
	menuSelected      int
	isDragging        bool
	lastClickX        int
	lastClickY        int
	lastClickTime     time.Time
	marksX            [26]int
	marksY            [26]int
	markSet           [26]bool
	registers         [256]reg
	undo              []undoState
	redo              []undoState
	countPrefix       int
	syntax            *syntax
	lastRows          []*row
	lastRowoff        int
	lastColoff        int
	lastChange        []int
	recordingChange   bool
	currentChange     []int
	keyBuffer         []int
}

var E editor
var Version = "dev"

var findLastMatch = -1
var findDirection = 1
var screenBuf bytes.Buffer

type winsize = unix.Winsize
