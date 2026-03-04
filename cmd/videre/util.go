package main

import (
	"fmt"
	"os"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

func die(err error) {
	disableRawMode()
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func utf8PrevBoundary(s []byte, idx int) int {
	if idx <= 0 {
		return 0
	}
	if idx > len(s) {
		idx = len(s)
	}
	idx--
	for idx > 0 && (s[idx]&0xC0) == 0x80 {
		idx--
	}
	return idx
}

func utf8NextBoundary(s []byte, idx int) int {
	if idx < 0 {
		return 0
	}
	if idx >= len(s) {
		return len(s)
	}
	for idx < len(s) && (s[idx]&0xC0) == 0x80 {
		idx++
	}
	if idx >= len(s) {
		return len(s)
	}
	_, n := utf8.DecodeRune(s[idx:])
	if n <= 0 {
		return idx + 1
	}
	if idx+n > len(s) {
		return len(s)
	}
	return idx + n
}

func utf8SnapBoundary(s []byte, idx int) int {
	if idx <= 0 {
		return 0
	}
	if idx >= len(s) {
		return len(s)
	}
	for idx > 0 && (s[idx]&0xC0) == 0x80 {
		idx--
	}
	return idx
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

func isAlphaByte(c byte) bool {
	l := c | 0x20
	return l >= 'a' && l <= 'z'
}

func isWordByte(c byte) bool { return isAlphaByte(c) || isDigitByte(c) || c == '_' }
func isWordChar(c byte) bool { return isWordByte(c) }

func ioctlGetWinsize(fd int, req uintptr) (*winsize, error) {
	return unix.IoctlGetWinsize(fd, uint(req))
}

func ioctlGetTermios(fd int, req uintptr) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, uint(req))
}

func ioctlSetTermios(fd int, req uintptr, t *unix.Termios) error {
	return unix.IoctlSetTermios(fd, uint(req), t)
}

func safeTermByte(c byte) byte {
	if c < 0x20 || c == 0x7f {
		return '?'
	}
	return c
}

func safeTermString(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	out := make([]byte, len(b))
	for i := range b {
		out[i] = safeTermByte(b[i])
	}
	return string(out)
}
