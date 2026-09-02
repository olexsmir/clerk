package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestServer_SelectionRange_DocumentNotFound(t *testing.T) {
	srv := newServer(t)
	res, err := srv.server.SelectionRange(t.Context(), &protocol.SelectionRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///nonexistent.journal")},
		Positions:    []protocol.Position{{Line: 0, Character: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("got %v, want nil", res)
	}
}

func TestGolden_SelectionRange(t *testing.T) {
	ar := golden.Read(t, "selection-range")
	h := newTxtarHarness(t, ar)
	li := lsputil.NewLineIndex(h.content)

	var b strings.Builder
	for i := range h.cursors {
		pos := lsputil.Position(h.content, h.cursors[i])
		res, err := h.srv.SelectionRange(t.Context(), &protocol.SelectionRangeParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: h.uri},
			Positions:    []protocol.Position{pos},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != 1 {
			t.Fatalf("cursor %d: got %d results, want 1", i, len(res))
		}
		chain := make([]string, 0, 4)
		for cur := &res[0]; ; cur = cur.Parent {
			if cur.Parent == nil {
				chain = append(chain, "<doc>")
				break
			}
			start := li.Offset(int(cur.Range.Start.Line), int(cur.Range.Start.Character))
			end := li.Offset(int(cur.Range.End.Line), int(cur.Range.End.Character))
			chain = append(chain, fmt.Sprintf("%q", h.content[start:end]))
		}
		fmt.Fprintf(&b, "%d:%d %s\n", pos.Line, pos.Character, strings.Join(chain, " -> "))
	}
	golden.Assert(t, ar, b.String())
}

func BenchmarkSelectionRange(b *testing.B) {
	path := "../../journal/testdata/journals/actual-1ktxns-100accts.journal"
	abs, err := filepath.Abs(path)
	if err != nil {
		b.Fatal(err)
	}
	rj, err := journal.NewLoader().Resolve(abs)
	if err != nil {
		b.Fatal(err)
	}
	content := string(rj.Occurrences[0].Src)

	srv := newServer(b)
	u := uri.File(abs)
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.analysisFor(u) // warm the per-doc cache

	for tname, off := range map[string]int{
		"account seg, tx 3, early":  strings.Index(content, "  1:2:3:4:5 ") + 5,
		"date, tx 1, early":         strings.Index(content, "2000-01-01 ") + 3,
		"account seg, tx 999, late": strings.Index(content, "  5b:5c:5d:5e:5f:60:61 ") + 6,
		"date, tx 999, late":        strings.Index(content, "2002-09-25 ") + 3,
	} {
		b.Run(tname, func(b *testing.B) {
			params := &protocol.SelectionRangeParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Positions:    []protocol.Position{lsputil.Position(content, off)},
			}
			// warm up: first request resolves the lazily-built analysis; assert it found one
			res, err := srv.server.SelectionRange(b.Context(), params)
			if err != nil {
				b.Fatal(err)
			}
			if len(res) != 1 {
				b.Fatalf("%s: got %d results, want 1", tname, len(res))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.SelectionRange(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
