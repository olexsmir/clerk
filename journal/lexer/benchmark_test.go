package lexer

import (
	"os"
	"testing"

	"olexsmir.xyz/clerk/journal/token"
)

func BenchmarkLexer(b *testing.B) {
	b.Run("basic", bench("../../journal/testdata/journals/basic.journal"))
	b.Run("personal", bench("../../journal/testdata/journals/actual-personal.journal"))
	b.Run("sample", bench("../../journal/testdata/journals/actual-sample.journal"))
	b.Run("wow", bench("../../journal/testdata/journals/actual-ledger-input-wow.dat"))
	b.Run("1k txns", bench("../../journal/testdata/journals/actual-1ktxns-100accts.journal"))
}

func bench(fpath string) func(b *testing.B) {
	src, err := os.ReadFile(fpath)
	if err != nil {
		panic("loadTestdata: " + err.Error())
	}

	return func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			l := New("bench", src)
			for l.Next().Type != token.EOF {
			}
		}
	}
}
