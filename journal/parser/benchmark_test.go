package parser

import (
	"os"
	"testing"

	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/token"
)

func loadTestdata(path string) []byte {
	src, err := os.ReadFile(path)
	if err != nil {
		panic("loadTestdata: " + err.Error())
	}
	return src
}

var (
	basicSrc     = loadTestdata("../../journal/testdata/journals/basic.journal")
	personalSrc  = loadTestdata("../../journal/testdata/journals/actual-personal.journal")
	sampleSrc    = loadTestdata("../../journal/testdata/journals/actual-sample.journal")
	standardSrc  = loadTestdata("../../journal/testdata/journals/actual-ledger-input-standard.dat")
	wowSrc       = loadTestdata("../../journal/testdata/journals/actual-ledger-input-wow.dat")
	benchmarkSrc = loadTestdata("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
)

func BenchmarkLexer_Basic(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", basicSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkLexer_Personal(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", personalSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkLexer_Sample(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", sampleSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkLexer_Standard(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", standardSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkLexer_Wow(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", wowSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkLexer_1kTxns(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", benchmarkSrc)
		for l.Next().Type != token.EOF {
		}
	}
}

func BenchmarkParser_Basic(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", basicSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_Personal(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", personalSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_Sample(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", sampleSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_Standard(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", standardSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_Wow(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", wowSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_1kTxns(b *testing.B) {
	for b.Loop() {
		l := lexer.New("bench", benchmarkSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_1kTxns_Allocs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		l := lexer.New("bench", benchmarkSrc)
		New(l).ParseJournal()
	}
}

func BenchmarkParser_Parallel_1kTxns(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l := lexer.New("bench", benchmarkSrc)
			New(l).ParseJournal()
		}
	})
}
