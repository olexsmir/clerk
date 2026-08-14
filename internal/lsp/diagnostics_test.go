package lsp

import (
	"testing"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal"
)

func BenchmarkDiagnostics(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	srv := NewServer("test")

	// Per-edit cost: a fresh loader skips the parse cache, so each iteration re-parses, then lints and groups findings.
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		an := analyzer.Build(journal.NewLoader().ResolveBytes("/test.journal", []byte(content)))
		finds := dedupFinds(srv.server.linter.Run(an))
		_ = srv.server.groupFindsByFile(finds)
	}
}
