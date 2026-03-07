package main

import (
	"fmt"
	"os"
	"unicode/utf8"
)

func die(err error) {
	if E.Screen != nil {
		E.Screen.Fini()
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

func utf8PrevBoundary(s []byte, idx int) int {
	if idx <= 0 { return 0 }
	if idx > len(s) { idx = len(s) }
	idx--
	for idx > 0 && (s[idx]&0xC0) == 0x80 { idx-- }
	return idx
}

func utf8NextBoundary(s []byte, idx int) int {
	if idx < 0 { return 0 }
	if idx >= len(s) { return len(s) }
	for idx < len(s) && (s[idx]&0xC0) == 0x80 { idx++ }
	if idx >= len(s) { return len(s) }
	_, n := utf8.DecodeRune(s[idx:])
	if n <= 0 { return idx + 1 }
	if idx+n > len(s) { return len(s) }
	return idx + n
}

func utf8SnapBoundary(s []byte, idx int) int {
	if idx <= 0 { return 0 }
	if idx >= len(s) { return len(s) }
	for idx > 0 && (s[idx]&0xC0) == 0x80 { idx-- }
	return idx
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }
func isAlphaByte(c byte) bool {
	l := c | 0x20
	return l >= 'a' && l <= 'z'
}
func isWordByte(c byte) bool { return isAlphaByte(c) || isDigitByte(c) || c == '_' }
func isWordChar(c byte) bool { return isWordByte(c) }

// Stub ioctl functions for compatibility if needed elsewhere
func ioctlGetWinsize(fd int, req uintptr) (struct{Row, Col uint16}, error) {
	return struct{Row, Col uint16}{}, nil
}
func ioctlGetTermios(fd int, req uintptr) (interface{}, error) {
	return nil, nil
}
