package analyzer

import (
	"testing"

	"olexsmir.xyz/clerk/journal"
)

func BenchmarkBuild(b *testing.B) {
	b.Run("basic", bench("../../journal/testdata/journals/basic.journal"))
	b.Run("standard", bench("../../journal/testdata/journals/actual-ledger-input-standard.dat"))
	b.Run("personal", bench("../../journal/testdata/journals/actual-personal.journal"))
	b.Run("1k txns", bench("../../journal/testdata/journals/actual-1ktxns-100accts.journal"))
	b.Run("wow", bench("../../journal/testdata/journals/actual-ledger-input-wow.dat"))
	b.Run("1k txns, prefix lookup", func(b *testing.B) {
		rj := loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
		a := Build(rj)
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
	})

	b.Run("1k txns, account iter", func(b *testing.B) {
		rj := loadJournal("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
		a := Build(rj)
		names := a.AccountNames
		b.ResetTimer()
		for b.Loop() {
			for _, name := range names {
				_ = a.Accounts[name].UsedCount
			}
		}
	})
}

func bench(fpath string) func(b *testing.B) {
	files := loadJournal(fpath)
	return func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			Build(files)
		}
	}
}

func loadJournal(path string) *journal.ResolvedJournal {
	ldr := journal.NewLoader()
	rj, err := ldr.Resolve(path)
	if err != nil {
		panic("loadJournal: " + err.Error())
	}
	return rj
}
