package lsputil

import (
	"testing"

	"olexsmir.xyz/clerk/journal/token"
)

// TestLineIndexParity asserts Offset/Position agree with the standalone
// converters, including clamped out-of-range positions.
func TestLineIndexParity(t *testing.T) {
	contents := []string{"abc\ndef\n", "abc\ndef", "abc", "a\nb\nc", "", "один\nдва\n"}
	for _, content := range contents {
		li := NewLineIndex(content)
		for _, line := range []int{0, 1, 2, 3, 100} {
			for _, col := range []int{0, 1, 5} {
				off := li.Offset(line, col)
				if want := Offset(content, line, col); off != want {
					t.Errorf("content %q: Offset(%d,%d) = %d, want %d", content, line, col, off, want)
				}
				pos := li.Position(off)
				if want := Position(content, off); pos != want {
					t.Errorf("content %q: Position(%d) = %+v, want %+v", content, off, pos, want)
				}
			}
		}
	}
}

func TestLineIndexSpanRange(t *testing.T) {
	content := "2024-01-15 grocery\n    expenses:food  $50\n    assets:cash\n"
	li := NewLineIndex(content)
	// span ending at the start of the next line clamps to the line end
	span := token.Span{
		Start: token.Pos{Offset: 11, Line: 1, Col: 12},
		End:   token.Pos{Offset: 18, Line: 2, Col: 0},
	}
	rng := li.SpanRange(span)
	if rng.Start.Line != 0 || rng.Start.Character != 11 || rng.End.Line != 0 || rng.End.Character != 18 {
		t.Errorf("grocery span = %+v, want 0:11-0:18", rng)
	}
}
