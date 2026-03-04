package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
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
	E = editor{mode: modeNormal, selSX: -1, selSY: -1, quitWarnRemaining: 1, menuSelected: 0}
	initContextMenuMetrics()
	updateWindowSize()
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			disableRawMode()
			fmt.Fprintf(os.Stderr, "videre panic: %v\n", r)
			_, _ = os.Stderr.Write(debug.Stack())
			os.Exit(2)
		}
	}()

	initEditor()

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

	if _, err := ioctlGetTermios(int(os.Stdin.Fd()), syscall.TCGETS); err != nil {
		fmt.Fprintln(os.Stderr, "videre requires a TTY on stdin")
		os.Exit(1)
	}
	if _, err := ioctlGetWinsize(int(os.Stdout.Fd()), syscall.TIOCGWINSZ); err != nil {
		fmt.Fprintln(os.Stderr, "videre requires a TTY on stdout")
		os.Exit(1)
	}
	enableRawMode()
	defer disableRawMode()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	go func() {
		for range sig {
			updateWindowSize()
			atomic.StoreInt32(&resizePending, 1)
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
