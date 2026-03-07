package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gdamore/tcell/v2"
)

const versionBanner = ` ┌──────────────────────────────────────────────────────────────┐
 │                                                              │
 │   __     __           ____  U _____ u   ____    U _____ u    │
 │  \ \   /"/u  ___    |  _"\ \| ___"|/U |  _"\ u \| ___"|/     │
 │   \ \ / //  |_"_|  /| | | | |  _|"   \| |_) |/  |  _|"       │
 │   /\ V /_,-. | |   U| |_| |\| |___    |  _ <    | |___       │
 │  U  \_/-(_/U/| |\u  |____/ u|_____|   |_| \_\   |_____|      │
 │    //   .-,_|___|_,-.|||_   <<   >>   //   \\_  <<   >>      │
 │   (__)   \_)-' '-(_/(__)_) (__) (__) (__)  (__)(__) (__)     │
 │                                                              │
 └──────────────────────────────────────────────────────────────┘`

func initEditor() {
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	s.SetStyle(defStyle)
	s.EnableMouse()
	s.EnablePaste()

	E = editor{
		Screen:            s,
		mode:              modeNormal,
		selSX:             -1,
		selSY:             -1,
		quitWarnRemaining: 1,
		menuSelected:      0,
	}

	w, h := s.Size()
	E.screenCols = w
	E.screenRows = h - 2 // Status bar and message bar
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-V" {
			fmt.Println(versionBanner)
			versionStr := fmt.Sprintf("videre %s", Version)
			padding := (64 - len(versionStr)) / 2
			if padding < 0 {
				padding = 0
			}
			fmt.Printf("%s%s\n", strings.Repeat(" ", padding), versionStr)
			os.Exit(0)
		}
	}

	initEditor()
	defer E.Screen.Fini()

	defer func() {
		if r := recover(); r != nil {
			E.Screen.Fini()
			fmt.Fprintf(os.Stderr, "videre panic: %v\n", r)
			_, _ = os.Stderr.Write(debug.Stack())
			os.Exit(2)
		}
	}()

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) >= 1 {
		_ = openFile(args[0])
	}

	refreshScreen()
	for {
		if processKeypress() {
			refreshScreen()
		}
	}
}
