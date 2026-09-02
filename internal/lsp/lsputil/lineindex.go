package lsputil

import (
	"sort"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/journal/token"
)

// LineIndex resolves byte offsets to LSP positions (and back) in O(log n) from
// a precomputed table of line starts, avoiding a per-request content scan.
type LineIndex struct {
	content string
	starts  []int // byte offset of each line's first byte; starts[0] == 0
}

// NewLineIndex builds the line-start table for content.
func NewLineIndex(content string) *LineIndex {
	starts := make([]int, 1, len(content)/20+1)
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{content: content, starts: starts}
}

// Position converts a byte offset to a 0-based LSP position.
func (l *LineIndex) Position(offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(l.content) {
		offset = len(l.content)
	}
	idx := sort.Search(len(l.starts), func(i int) bool { return l.starts[i] > offset }) - 1
	lineStart := l.starts[idx]
	return protocol.Position{
		Line:      uint32(idx),
		Character: uint32(Utf16Col(l.content[lineStart:offset], offset-lineStart)),
	}
}

// Offset converts a 0-based line and UTF-16 code unit column to a byte offset,
// clamped to the content bounds. The inverse of Position; matches the
// standalone Offset on the same content.
func (l *LineIndex) Offset(line, col int) int {
	if line < 0 {
		line = 0
	}
	if line >= len(l.starts) {
		return len(l.content)
	}
	lineStart := l.starts[line]
	lineEnd := len(l.content)
	if line+1 < len(l.starts) {
		lineEnd = l.starts[line+1]
	}
	for lineEnd > lineStart && (l.content[lineEnd-1] == '\n' || l.content[lineEnd-1] == '\r') {
		lineEnd--
	}
	seg := l.content[lineStart:lineEnd]
	ascii := true
	for i := range seg {
		if seg[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		if col >= len(seg) {
			return lineEnd
		}
		return lineStart + col
	}
	off := lineStart
	units := 0
	for off < lineEnd && units < col {
		r, size := utf8.DecodeRuneInString(l.content[off:lineEnd])
		off += size
		units += utf16Len(r)
	}
	return off
}

// SpanRange converts a span to a protocol range, trimming trailing whitespace and newlines from the end.
func (l *LineIndex) SpanRange(span token.Span) protocol.Range {
	return protocol.Range{
		Start: l.Position(span.Start.Offset),
		End:   l.Position(l.clampEnd(span.End.Offset)),
	}
}

func (l *LineIndex) clampEnd(end int) int {
	for end > 0 {
		switch l.content[end-1] {
		case ' ', '\t', '\r', '\n':
			end--
		default:
			return end
		}
	}
	return end
}
