package lsp

import (
	"os"
	"testing"

	"go.lsp.dev/uri"
)

func BenchmarkDiagnostics(b *testing.B) {
	path := "../../journal/testdata/journals/actual-1ktxns-100accts.journal"
	src, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), string(src), 1, "journal")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Per-edit cost: re-resolve (parse) + lint + group findings.
		a := srv.server.buildAnalysis()
		finds := dedupFinds(srv.server.linter.Run(a))
		_ = srv.server.groupFindsByFile(finds)
	}
}
