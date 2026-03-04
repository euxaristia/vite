package main

import (
	"os"
	"os/exec"
	"testing"
)

func checkNvim(b *testing.B) {
	if _, err := exec.LookPath("nvim"); err != nil {
		b.Skip("nvim not found, skipping comparison")
	}
}

func runComparison(b *testing.B, testFunc func(b *testing.B, d EditorDriver)) {
	b.Helper()
	
	b.Run("VIDERE", func(b *testing.B) {
		d := NewVidereDriver()
		testFunc(b, d)
	})

	checkNvim(b)
	b.Run("NVIM", func(b *testing.B) {
		d := NewNvimDriver()
		testFunc(b, d)
	})

	b.Run("VIM", func(b *testing.B) {
		d := NewVimDriver()
		testFunc(b, d)
	})
}

func BenchmarkCompareStartup(b *testing.B) {
	runComparison(b, func(b *testing.B, d EditorDriver) {
		for i := 0; i < b.N; i++ {
			_, err := d.Start("")
			if err != nil {
				b.Fatal(err)
			}
			_, _ = d.Quit(true)
		}
	})
}

func BenchmarkCompareInsertion(b *testing.B) {
	runComparison(b, func(b *testing.B, d EditorDriver) {
		for i := 0; i < b.N; i++ {
			_, err := d.Start("")
			if err != nil {
				b.Fatal(err)
			}
			_ = d.SendKeys("i", 0)
			for j := 0; j < 10; j++ {
				_ = d.SendKeys("hello ", 0)
			}
			_ = d.SendKeys("<ESC>", 0)
			if _, err := d.Quit(true); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompareSyntax(b *testing.B) {
	// Generate a large C file with lots of comments
	f, err := os.CreateTemp("", "bench_syntax_*.c")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(f.Name())
	
	f.WriteString("/* Start of large comment block\n")
	for i := 0; i < 5000; i++ {
		f.WriteString("   ... middle of comment ...\n")
	}
	f.WriteString("   End of comment block */\n")
	for i := 0; i < 5000; i++ {
		f.WriteString("int main() { return 0; } // basic code\n")
	}
	f.Close()

	runComparison(b, func(b *testing.B, d EditorDriver) {
		for i := 0; i < b.N; i++ {
			_, err := d.Start(f.Name())
			if err != nil {
				b.Fatal(err)
			}
			// Scroll down a bit to trigger syntax updates
			for j := 0; j < 5; j++ {
				_ = d.SendKeys("L", 0) // Go to bottom of screen (if supported) or just move
				_ = d.SendKeys("50j", 0)
			}
			if _, err := d.Quit(true); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompareMovement(b *testing.B) {
	// Generate a temporary file for movement tests
	f, err := os.CreateTemp("", "bench_move_*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(f.Name())
	for i := 0; i < 100; i++ {
		f.WriteString("The quick brown fox jumps over the lazy dog.\n")
	}
	f.Close()

	runComparison(b, func(b *testing.B, d EditorDriver) {
		for i := 0; i < b.N; i++ {
			_, err := d.Start(f.Name())
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < 10; j++ {
				_ = d.SendKeys("jjjkkkwwbbe", 0)
			}
			if _, err := d.Quit(true); err != nil {
				b.Fatal(err)
			}
		}
	})
}
