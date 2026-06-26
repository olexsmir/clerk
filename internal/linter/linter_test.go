package linter

import (
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

var tests = map[string][]Rule{
	"correct":                  Rules,
	"empty-postings":           {&EmptyPostings{}},
	"parse-error":              {&ParseError{}},
	"omitted-precision":        {&OmittedPrecision{}},
	"missing-commodity":        {&MissingCommodity{}},
	"missing-status":           {&MissingStatus{}},
	"account-depth":            {&AccountDepthLimit{MaxDepth: 3}},
	"multiple-omitted-amounts": {&MultipleOmittedAmounts{}},
	"orderdate":                {&OrderDate{}},
	"duplicated-account":       {&DuplicatedAccount{}},
	"duplicated-commodity":     {&DuplicatedCommodity{}},
	"undeclared-account":       {&UndeclaredAccount{}},
}

func TestLinter(t *testing.T) {
	for tname, trules := range tests {
		t.Run(tname, func(t *testing.T) {
			inp := golden.Load(t, tname)
			pf, err := journal.NewLoader().LoadBytes(tname+".journal", inp)
			if err != nil {
				t.Fatalf("failed to load test journal: %v\n", err)
			}

			finds := NewLinter(trules).Run(pf.Ast)

			var b strings.Builder
			Fprint(&b, PathBasename, finds)
			golden.Assert(t, tname, b.String())
		})
	}
}

func BenchmarkLinter(b *testing.B) {
	pf, err := journal.NewLoader().Load(
		"../../journal/testdata/journals/actual-1ktxns-100accts.journal",
	)
	if err != nil {
		b.Fatalf("failed to load benchmark journal: %v", err)
	}
	l := NewLinter(Rules)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		l.Run(pf.Ast)
	}
}
