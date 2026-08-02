package lsputil

import (
	"fmt"
	"testing"
	"unicode/utf8"
)

func TestUtf16Col(t *testing.T) {
	tests := map[string]struct {
		line   string
		offset int
		want   int
	}{
		"ascii":              {"hello", 3, 3},
		"cyrillic full line": {"Привіт", len("Привіт"), 6},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Utf16Col(tt.line, tt.offset); got != tt.want {
				t.Errorf("Utf16Col(%q, %d) = %d, want %d", tt.line, tt.offset, got, tt.want)
			}
		})
	}
}

func TestUtf16Len(t *testing.T) {
	tests := map[string]struct {
		content string
		offset  int
		end     int
		want    int
	}{
		"ascii":    {"hello world", 0, 5, 5},
		"cyrillic": {"Привіт світ", 0, len("Привіт"), 6},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Utf16Len(tt.content, tt.offset, tt.end); got != tt.want {
				t.Errorf("Utf16Len(%q, %d, %d) = %d, want %d", tt.content, tt.offset, tt.end, got, tt.want)
			}
		})
	}
}

func TestLineCol(t *testing.T) {
	tests := map[string]struct {
		content   string
		offset    int
		line, col int
	}{
		"basic":             {"hello\nworld", 11, 1, 5},
		"first line":        {"hello world", 5, 0, 5},
		"line start":        {"ab\ncd", 3, 1, 0},
		"newline ends line": {"ab\ncd", 2, 0, 2},
		"past blank line":   {"a\n\nb", 3, 2, 0},
		"blank line col":    {"a\n\nb", 2, 1, 0},
		"CRLF line start":   {"ab\r\ncd", 4, 1, 0},
		"crlf newline ends": {"ab\r\ncd", 2, 0, 2},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			line, col := LineCol(tt.content, tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("LineCol(%q, %d) = (%d,%d), want (%d,%d)", tt.content, tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestOffset(t *testing.T) {
	tests := map[string]struct {
		content   string
		line, col int
		want      int
	}{
		"basic":        {"hello\nworld", 1, 5, 11},
		"past EOL col": {"ab\ncd", 0, 99, 2},
		"cyrillic":     {"Привіт😀", 0, 6, 12},
		"emoji":        {"Привіт😀", 0, 7, 16},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Offset(tt.content, tt.line, tt.col); got != tt.want {
				t.Errorf("Offset(%d,%d) = %d, want %d", tt.line, tt.col, got, tt.want)
			}
		})
	}

	t.Run("past EOF line", func(t *testing.T) {
		if got := Offset("ab\ncd", 5, 0); got != len("ab\ncd") {
			t.Errorf("Offset = %d, want %d", got, len("ab\ncd"))
		}
	})
}

func TestOffset_RoundTrip(t *testing.T) {
	content := "first\nПривіт світ\r\nlast\n"

	var boundaries []int
	for i := 0; i < len(content); {
		_, size := utf8.DecodeRuneInString(content[i:])
		if content[i] != '\n' || i <= 0 || content[i-1] != '\r' {
			boundaries = append(boundaries, i)
		}
		i += size
	}

	for _, off := range boundaries {
		t.Run(fmt.Sprintf("round trip at %d", off), func(t *testing.T) {
			line, col := LineCol(content, off)
			if got := Offset(content, line, col); got != off {
				t.Errorf("round trip at %d: Offset(LineCol(%d)) = %d", off, off, got)
			}
		})
	}
}

func TestPosition(t *testing.T) {
	content := "2024-01-15 Супермаркет\r\n  Витрати:Продукти  ¥50\n"
	for _, off := range []int{0, 10, len("2024-01-15 Супермаркет"), len(content)} {
		t.Run(fmt.Sprintf("round trip at %d", off), func(t *testing.T) {
			p := Position(content, off)
			if got := Offset(content, int(p.Line), int(p.Character)); got != off {
				t.Errorf("round trip of %d = %d (pos %d:%d), want identity", off, got, p.Line, p.Character)
			}
		})
	}
}
