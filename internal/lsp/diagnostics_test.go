package lsp

import (
	"testing"

	"go.lsp.dev/uri"
)

func BenchmarkDiagnostics(b *testing.B) {
	content := openJouranl(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Per-edit cost: re-resolve (parse) + lint + group findings.
		a := srv.server.buildAnalysis()
		finds := dedupFinds(srv.server.linter.Run(a))
		_ = srv.server.groupFindsByFile(finds)
	}
}
