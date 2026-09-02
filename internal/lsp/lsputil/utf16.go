package lsputil

import (
	"unicode/utf8"

	"go.lsp.dev/protocol"
)

// Utf16Col returns the UTF-16 code unit column (0-based) for a byte offset
// within the given line content (without newline). Pure-ASCII prefixes score
// one unit per byte without decoding.
func Utf16Col(line string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset > len(line) {
		byteOffset = len(line)
	}
	asc := line[:byteOffset]
	for i := range asc {
		if asc[i] >= utf8.RuneSelf {
			return utf16ColSlow(line, byteOffset)
		}
	}
	return byteOffset
}

func utf16ColSlow(line string, byteOffset int) int {
	col := 0
	for i := 0; i < byteOffset; {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		col += utf16Len(r)
		i += size
	}
	return col
}

// Utf16ColBytes returns the UTF-16 code unit column of b (without newline).
func Utf16ColBytes(b []byte) int {
	for _, c := range b {
		if c >= utf8.RuneSelf {
			return utf16ColBytesSlow(b)
		}
	}
	return len(b)
}

func utf16ColBytesSlow(b []byte) int {
	col := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		col += utf16Len(r)
		i += size
	}
	return col
}

// Utf16Len returns the UTF-16 code unit length of content[offset:end].
func Utf16Len(content string, offset, end int) int {
	if offset < 0 {
		offset = 0
	}
	if end > len(content) {
		end = len(content)
	}
	if offset >= end {
		return 0
	}
	seg := content[offset:end]
	for i := range seg {
		if seg[i] >= utf8.RuneSelf {
			return utf16LenSlow(content, offset, end)
		}
	}
	return end - offset
}

func utf16LenSlow(content string, offset, end int) int {
	n := 0
	for i := offset; i < end; {
		r, size := utf8.DecodeRuneInString(content[i:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		n += utf16Len(r)
		i += size
	}
	return n
}

// Offset converts a 0-based line and UTF-16 code unit column to a byte offset
// in content, clamped to the content bounds. The inverse of LineCol.
func Offset(content string, line, col int) int {
	if line < 0 {
		line = 0
	}
	off := 0
	for curLine := 0; curLine < line && off < len(content); {
		switch content[off] {
		case '\n':
			off++
			curLine++
		case '\r':
			off++
			if off < len(content) && content[off] == '\n' {
				off++
			}
			curLine++
		default:
			for off < len(content) && content[off] != '\n' && content[off] != '\r' {
				off++
			}
		}
	}
	lineEnd := off
	for lineEnd < len(content) && content[lineEnd] != '\n' && content[lineEnd] != '\r' {
		lineEnd++
	}
	units := 0
	for off < lineEnd && units < col {
		r, size := utf8.DecodeRuneInString(content[off:])
		off += size
		units += utf16Len(r)
	}
	return off
}

// LineCol converts a byte offset in content to 0-based line number and
// UTF-16 code unit column.
func LineCol(content string, offset int) (line int, col int) {
	if offset <= 0 {
		return 0, 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	lineStart := 0
	lineNum := 0
	for i := 0; i < len(content); {
		r, size := utf8.DecodeRuneInString(content[i:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if r == '\n' {
			if offset < i+size {
				col = Utf16Col(content[lineStart:offset], offset-lineStart)
				return lineNum, col
			}
			lineStart = i + size
			lineNum++
			i += size
			continue
		}
		if r == '\r' {
			// skip \r\n
			if i+1 < len(content) && content[i+1] == '\n' {
				size++
			}
			if offset < i+size {
				col = Utf16Col(content[lineStart:offset], offset-lineStart)
				return lineNum, col
			}
			lineStart = i + size
			lineNum++
			i += size
			continue
		}
		if i+size > offset {
			col = Utf16Col(content[lineStart:offset], offset-lineStart)
			return lineNum, col
		}
		i += size
	}
	col = Utf16Col(content[lineStart:], len(content)-lineStart)
	return lineNum, col
}

// Position converts a byte offset to an LSP position (0-based line, UTF-16
// code unit column). The inverse of Offset at the protocol.Position level.
func Position(content string, offset int) protocol.Position {
	line, col := LineCol(content, offset)
	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}

// utf16Len returns the number of UTF-16 code units for a rune.
func utf16Len(r rune) int {
	if r >= 0x10000 && r <= 0x10FFFF {
		return 2
	}
	return 1
}
