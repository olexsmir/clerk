package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
)

func TestServer_References_DocumentNotFound(t *testing.T) {
	srv := NewServer("test")
	res, err := srv.server.References(context.Background(), &protocol.ReferenceParams{
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

func TestGolden_References(t *testing.T) {
	for _, tt := range []string{"references-journal", "references-include"} {
		ar := golden.Read(t, tt)
		t.Run(tt, func(t *testing.T) {
			h := newTxtarHarness(t, ar)

			var b strings.Builder
			for _, incl := range []bool{false, true} {
				fmt.Fprintf(&b, "includeDeclaration:%v\n", incl)
				for i := range h.cursors {
					txtDocPos := h.textDocumentPosition(i)
					res, err := h.srv.References(t.Context(), &protocol.ReferenceParams{
						Context:                    protocol.ReferenceContext{IncludeDeclaration: incl},
						TextDocumentPositionParams: txtDocPos,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(res) == 0 {
						fmt.Fprintf(&b, "%d:%d <none>\n", txtDocPos.Position.Line, txtDocPos.Position.Character)
						continue
					}
					for _, loc := range res {
						r := loc.Range
						fmt.Fprintf(&b, "%d:%d %s %d:%d-%d:%d\n", txtDocPos.Position.Line, txtDocPos.Position.Character,
							filepath.Base(loc.URI.Path()),
							r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
					}
				}
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

func BenchmarkReferences(b *testing.B) {
	content := openJouranl(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := NewServer("test")
	u := uri.URI("file:///test.journal")
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.analysisFor(u) // warm per-doc cache

	for tname, tt := range map[string]int{
		"1k txns, account":   strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2,
		"1k txns, payee":     strings.Index(content, "transaction 1") + len("transaction "),
		"1k txns, commodity": strings.Index(content, "2 B @@") + len("2 B"),
		"late tx, payee":     strings.Index(content, "transaction 1000") + len("transaction "),
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt)
			params := &protocol.ReferenceParams{
				Context: protocol.ReferenceContext{IncludeDeclaration: true},
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: u},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
			}

			// warm up: first request resolves the symbol lazily; assert it found references
			res, err := srv.server.References(b.Context(), params)
			if err != nil {
				b.Fatal(err)
			}
			if len(res) == 0 {
				b.Fatalf("%s: no references", tname)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.References(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}

			// guard: a whole-file re-parse per request (~13ms) would blow past
			// this and must be caught
			if avg := b.Elapsed() / time.Duration(b.N); avg > 5*time.Millisecond {
				b.Fatalf("references %v/op: reparse regression", avg)
			}
		})
	}
}
