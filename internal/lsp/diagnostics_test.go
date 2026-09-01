package lsp

import (
	"testing"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
)

func BenchmarkDiagnostics(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	srv := newServer(b)

	lint, err := linter.NewLinter(srv.server.settings.Linter)
	if err != nil {
		b.Fatal(err)
	}

	// Per-edit cost: a fresh loader skips the parse cache, so each iteration re-parses, then lints and groups findings.
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		an := analyzer.Build(journal.NewLoader().ResolveBytes("/test.journal", []byte(content)))
		finds := dedupFinds(lint.Run(an))
		srv.server.assignSeverities(finds)
		_ = srv.server.groupFindsByFile(finds)
	}
}
