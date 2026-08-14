package lsp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func TestTagValueSpan(t *testing.T) {
	tag := func(start, end int) *ast.Tag {
		return &ast.Tag{
			Span: token.Span{Start: token.Pos{Offset: start}, End: token.Pos{Offset: end}},
		}
	}

	tests := map[string]struct {
		content            string
		tag                *ast.Tag
		wantStart, wantEnd int
	}{
		"value":                {"; project:home", tag(2, 14), 10, 14},
		"empty value":          {"; done:", tag(2, 7), 7, 7},
		"trailing space":       {"; done: ", tag(2, 8), 7, 7},
		"second of comma list": {"; a:1, b:2", tag(7, 10), 9, 10},
	}
	for tname, tt := range tests {
		t.Run(tname, func(t *testing.T) {
			span := tagValueSpan(tt.content, tt.tag)
			if span.Start.Offset != tt.wantStart || span.End.Offset != tt.wantEnd {
				t.Errorf("got %d-%d, want %d-%d", span.Start.Offset, span.End.Offset, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestServer_Hover_DocumentNotFound(t *testing.T) {
	srv := NewServer("test")
	res, err := srv.server.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///nonexistent.journal")},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("got %v, want nil", res)
	}
}

// Golden

func TestGolden_Hover(t *testing.T) {
	for _, tt := range []string{"hover-journal", "hover-include"} {
		ar := golden.Read(t, tt)
		t.Run(tt, func(t *testing.T) {
			h := newTxtarHarness(t, ar)

			var b strings.Builder
			for i := range h.cursors {
				tdp := h.textDocumentPosition(i)
				res, err := h.srv.Hover(t.Context(), &protocol.HoverParams{
					TextDocumentPositionParams: tdp,
				})
				if err != nil {
					t.Fatal(err)
				}
				if res == nil {
					fmt.Fprintf(&b, "%d:%d <none>\n", tdp.Position.Line, tdp.Position.Character)
					continue
				}
				mc, ok := res.Contents.(*protocol.MarkupContent)
				if !ok {
					t.Fatalf("Hover returned %T, want *MarkupContent", res.Contents)
				}
				fmt.Fprintf(&b, "%d:%d %q %d:%d-%d:%d\n",
					tdp.Position.Line, tdp.Position.Character, mc.Value,
					res.Range.Start.Line, res.Range.Start.Character, res.Range.End.Line, res.Range.End.Character)
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

func BenchmarkHover(b *testing.B) {
	content := openJouranl(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
	srv.server.analysisFor(uri.URI("file:///test.journal")) // warm the per-doc cache

	for tname, tt := range map[string]int{
		"date":              strings.Index(content, "2000-01-01 transaction 1") + 3,
		"account":           strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2,
		"payee":             strings.Index(content, "transaction 1") + len("transaction ") + 1,
		"amount with cost":  strings.Index(content, "2 B @@") + 2,
		"commodity in cost": strings.Index(content, "@@ 2 C") + len("@@ 2 C") - 1,
		"late amount":       strings.Index(content, "1000 L @") + 2,
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt)
			params := &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
			}

			// warm up
			if _, err := srv.server.Hover(b.Context(), params); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.Hover(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}

			// guard: hover walks entries from the top of the file, so a cursor
			// near the end costs ~0.8ms on this file; a whole-file re-parse per
			// request (~13ms) would blow past this and must be caught
			if avg := b.Elapsed() / time.Duration(b.N); avg > 5*time.Millisecond {
				b.Fatalf("hover %v/op: entry walk or reparse regression", avg)
			}
		})
	}
}
