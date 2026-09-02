package lsp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/testutil/golden"
)

func TestGolden_FoldingRange(t *testing.T) {
	ar := golden.Read(t, "folding-range")
	h := newTxtarHarness(t, ar)

	res, err := h.srv.FoldingRange(t.Context(), &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: h.uri},
	})
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for _, r := range res {
		fmt.Fprintf(&b, "%d-%d %s\n", r.StartLine, r.EndLine, r.Kind)
	}
	golden.Assert(t, ar, b.String())
}

func BenchmarkFoldingRange(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := newServer(b)
	u := uri.URI("file:///test.journal")
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.analysisFor(u) // warm the per-doc cache

	params := &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	}

	// warm up: assert the ranges are populated
	res, err := srv.server.FoldingRange(b.Context(), params)
	if err != nil {
		b.Fatal(err)
	}
	if len(res) == 0 {
		b.Fatalf("FoldingRange returned 0 ranges")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := srv.server.FoldingRange(b.Context(), params); err != nil {
			b.Fatal(err)
		}
	}

	// guard: folding iterates the AST once per request; a whole-file
	// re-parse or quadratic scan must be caught
	if avg := b.Elapsed() / time.Duration(b.N); avg > 5*time.Millisecond {
		b.Fatalf("foldingRange %v/op: regression", avg)
	}
}
