package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func BenchmarkFormatting(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := newServer(b)
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")

	params := &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
	}
	// warm up: first request parses and prints
	if _, err := srv.server.Formatting(b.Context(), params); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := srv.server.Formatting(b.Context(), params); err != nil {
			b.Fatal(err)
		}
	}
}
