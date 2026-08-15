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

func TestGolden_DocumentSymbols(t *testing.T) {
	ar := golden.Read(t, "document-symbol")
	h := newTxtarHarness(t, ar)

	res, err := h.srv.DocumentSymbol(t.Context(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: h.uri},
	})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := res.(protocol.DocumentSymbolSlice)
	if !ok {
		t.Fatalf("DocumentSymbol returned %T, want DocumentSymbolSlice", res)
	}
	var b strings.Builder
	for _, sym := range list {
		r, sel := sym.Range, sym.SelectionRange
		fmt.Fprintf(&b, "%q %s %d:%d-%d:%d %d:%d-%d:%d\n", sym.Name, symbolKindName(sym.Kind),
			r.Start.Line, r.Start.Character, r.End.Line, r.End.Character,
			sel.Start.Line, sel.Start.Character, sel.End.Line, sel.End.Character)
	}
	golden.Assert(t, ar, b.String())
}

func BenchmarkDocumentSymbol(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := NewServer("test")
	u := uri.URI("file:///test.journal")
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.analysisFor(u) // warm the per-doc cache

	params := &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	}

	// warm up: assert the outline is populated
	res, err := srv.server.DocumentSymbol(b.Context(), params)
	if err != nil {
		b.Fatal(err)
	}
	list, ok := res.(protocol.DocumentSymbolSlice)
	if !ok || len(list) == 0 {
		b.Fatalf("DocumentSymbol returned %T with %d symbols, want non-empty slice", res, len(list))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := srv.server.DocumentSymbol(b.Context(), params); err != nil {
			b.Fatal(err)
		}
	}

	// guard: documentSymbol maps every entry per request; a whole-file
	// re-parse or quadratic scan must be caught
	if avg := b.Elapsed() / time.Duration(b.N); avg > 5*time.Millisecond {
		b.Fatalf("documentSymbol %v/op: regression", avg)
	}
}
