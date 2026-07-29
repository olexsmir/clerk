package lsputil

import "testing"

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
