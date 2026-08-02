package lsputil

import (
	"testing"
	"unicode/utf8"
)

func TestUtf16Col_Basic(t *testing.T) {
	col := Utf16Col("hello", 3)
	if col != 3 {
		t.Errorf("Utf16Col for ASCII = %d, want 3", col)
	}
}

func TestUtf16Col_Cyrillic(t *testing.T) {
	// "Привет" is 6 Cyrillic chars = 12 bytes, 6 UTF-16 code units
	line := "Привет"
	col := Utf16Col(line, len(line))
	if col != 6 {
		t.Errorf("Utf16Col for 'Привет' = %d, want 6", col)
	}
}

func TestUtf16Len_ASCII(t *testing.T) {
	l := Utf16Len("hello world", 0, 5)
	if l != 5 {
		t.Errorf("Utf16Len = %d, want 5", l)
	}
}

func TestUtf16Len_Cyrillic(t *testing.T) {
	// "Привет" = 6 UTF-16 code units
	l := Utf16Len("Привет мир", 0, 12)
	if l != 6 {
		t.Errorf("Utf16Len for 'Привет' = %d, want 6", l)
	}
}

func TestLineCol_Basic(t *testing.T) {
	line, col := LineCol("hello\nworld", 11)
	if line != 1 || col != 5 {
		t.Errorf("LineCol = (%d,%d), want (1,5)", line, col)
	}
}

func TestLineCol_FirstLine(t *testing.T) {
	line, col := LineCol("hello world", 5)
	if line != 0 || col != 5 {
		t.Errorf("LineCol = (%d,%d), want (0,5)", line, col)
	}
}

func TestLineCol_LineStart(t *testing.T) {
	// offset at the first char of line 1 must be (1,0), not the tail of line 0
	line, col := LineCol("ab\ncd", 3)
	if line != 1 || col != 0 {
		t.Errorf("LineCol(3) = (%d,%d), want (1,0)", line, col)
	}
	// newline char itself belongs to the line it ends
	line, col = LineCol("ab\ncd", 2)
	if line != 0 || col != 2 {
		t.Errorf("LineCol(2) = (%d,%d), want (0,2)", line, col)
	}
}

func TestLineCol_BlankLine(t *testing.T) {
	// "a\n\nb": line 1 is blank; offset 3 is the start of line 2
	line, col := LineCol("a\n\nb", 3)
	if line != 2 || col != 0 {
		t.Errorf("LineCol(3) = (%d,%d), want (2,0)", line, col)
	}
	line, col = LineCol("a\n\nb", 2)
	if line != 1 || col != 0 {
		t.Errorf("LineCol(2) = (%d,%d), want (1,0)", line, col)
	}
}

func TestLineCol_CRLF(t *testing.T) {
	line, col := LineCol("ab\r\ncd", 4)
	if line != 1 || col != 0 {
		t.Errorf("LineCol(4) = (%d,%d), want (1,0)", line, col)
	}
	line, col = LineCol("ab\r\ncd", 2)
	if line != 0 || col != 2 {
		t.Errorf("LineCol(2) = (%d,%d), want (0,2)", line, col)
	}
}

func TestOffset_Basic(t *testing.T) {
	if got := Offset("hello\nworld", 1, 5); got != 11 {
		t.Errorf("Offset(1,5) = %d, want 11", got)
	}
}

func TestOffset_RoundTrip(t *testing.T) {
	content := "first\nПривет мир\r\nlast\n"
	// only rune-boundary offsets round-trip (LineCol clamps mid-rune offsets)
	var boundaries []int
	for i := 0; i < len(content); {
		_, size := utf8.DecodeRuneInString(content[i:])
		if !(content[i] == '\n' && i > 0 && content[i-1] == '\r') {
			boundaries = append(boundaries, i)
		}
		i += size
	}
	for _, off := range boundaries {
		line, col := LineCol(content, off)
		if got := Offset(content, line, col); got != off {
			t.Errorf("round trip at %d: Offset(LineCol(%d)) = %d", off, off, got)
		}
	}
}

func TestOffset_Clamps(t *testing.T) {
	content := "ab\ncd"
	if got := Offset(content, 5, 0); got != len(content) {
		t.Errorf("past EOF line: Offset = %d, want %d", got, len(content))
	}
	// column beyond line end clamps to line end
	if got := Offset(content, 0, 99); got != 2 {
		t.Errorf("past EOL col: Offset = %d, want 2", got)
	}
}

func TestOffset_UTF16(t *testing.T) {
	// Cyrillic chars are 1 UTF-16 unit each; emoji are 2
	content := "Привет😀"
	if got := Offset(content, 0, 6); got != 12 {
		t.Errorf("Offset after Cyrillic = %d, want 12", got)
	}
	if got := Offset(content, 0, 7); got != 16 {
		t.Errorf("Offset after emoji (2 UTF-16 units) = %d, want 16", got)
	}
}

func TestPosition_RoundTrip(t *testing.T) {
	content := "2024-01-15 Супермаркет\r\n    Витрати:Продукти  ¥50\n"
	for _, off := range []int{0, 10, len("2024-01-15 Супермаркет"), len(content)} {
		p := Position(content, off)
		if got := Offset(content, int(p.Line), int(p.Character)); got != off {
			t.Errorf("round trip of %d = %d (pos %d:%d), want identity", off, got, p.Line, p.Character)
		}
	}
}
