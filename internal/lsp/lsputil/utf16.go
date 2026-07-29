package lsputil

import "unicode/utf8"

// Utf16Col returns the UTF-16 code unit column (0-based) for a byte offset
// within the given line content (without newline).
func Utf16Col(line string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset > len(line) {
		byteOffset = len(line)
	}
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

// utf16Len returns the number of UTF-16 code units for a rune.
func utf16Len(r rune) int {
	if r >= 0x10000 && r <= 0x10FFFF {
		return 2
	}
	return 1
}
