package analyzer

import (
	"testing"

	"olexsmir.xyz/clerk/journal"
)

func BenchmarkBuild_BasicJournal(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/basic.journal")
	b.ResetTimer()
	for b.Loop() {
		Build(files)
	}
}

func BenchmarkBuild_1kTxns(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	b.ResetTimer()
	for b.Loop() {
		Build(files)
	}
}

func BenchmarkBuild_Standard(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/actual-ledger-input-standard.dat")
	b.ResetTimer()
	for b.Loop() {
		Build(files)
	}
}

func BenchmarkBuild_Wow(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/actual-ledger-input-wow.dat")
	b.ResetTimer()
	for b.Loop() {
		Build(files)
	}
}

func BenchmarkBuild_1kTxns_Allocs(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		Build(files)
	}
}

func BenchmarkBuild_Parallel_1kTxns(b *testing.B) {
	files := loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Build(files)
		}
	})
}

func BenchmarkPrefixLookup_1kTxns(b *testing.B) {
	a := Build(loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal"))
	prefixes := make([]string, 0, len(a.AccountsByPrefix))
	for p := range a.AccountsByPrefix {
		prefixes = append(prefixes, p)
	}
	b.ResetTimer()
	for b.Loop() {
		for _, p := range prefixes {
			_ = a.AccountsByPrefix[p]
		}
	}
}

func BenchmarkAccountIteration_1kTxns(b *testing.B) {
	a := Build(loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal"))
	names := a.AccountNames
	b.ResetTimer()
	for b.Loop() {
		for _, name := range names {
			_ = a.Accounts[name].UsedCount
		}
	}
}

func loadJournal(path string) []*journal.ParsedFile {
	ldr := journal.NewLoader()
	if _, err := ldr.Load(path); err != nil {
		panic("loadJournal: " + err.Error())
	}
	return ldr.Ordered()
}
