package linter

import (
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/semantic"
)

var tests = map[string][]Rule{
	"correct":                  Rules,
	"empty-postings":           {&EmptyPostings{}},
	"parse-error":              {&ParseError{}},
	"omitted-precision":        {&OmittedPrecision{}},
	"missing-commodity":        {&MissingCommodity{}},
	"missing-status":           {&MissingStatus{}},
	"missing-payee":            {&MissingPayee{}},
	"account-depth":            {&AccountDepthLimit{MaxDepth: 3}},
	"multiple-omitted-amounts": {&MultipleOmittedAmounts{}},
	"orderdate":                {&OrderDate{}},
	"duplicated-account":       {&DuplicatedAccount{}},
	"duplicated-commodity":     {&DuplicatedCommodity{}},
	"undeclared-commodity":     {&UndeclaredCommodity{}},
	"undeclared-account":       {&UndeclaredAccount{}},
	"unbalanced-transaction":   {&UnbalancedTransaction{}},
	"unused-account":           {&UnusedAccount{}},
}

func TestLinter(t *testing.T) {
	for tname, trules := range tests {
		t.Run(tname, func(t *testing.T) {
			a := golden.Read(t, tname)
			fsys, err := a.FS()
			if err != nil {
				t.Fatal(err)
			}

			l := journal.NewLoader()
			if _, err := l.LoadFS(fsys, "in.journal"); err != nil {
				t.Fatalf("failed to load test journal: %v", err)
			}

			ctx := semantic.Build(l.Ordered())
			finds := NewLinter(trules).Run(ctx)

			var b strings.Builder
			Fprint(&b, PathBasename, finds)
			golden.Assert(t, a, b.String())
		})
	}
}

func BenchmarkLinter(b *testing.B) {
	ldr := journal.NewLoader()
	_, err := ldr.Load(
		"../../journal/testdata/journals/actual-1ktxns-100accts.journal",
	)
	if err != nil {
		b.Fatalf("failed to load benchmark journal: %v", err)
	}

	ctx := semantic.Build(ldr.Ordered())
	l := NewLinter(Rules)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		l.Run(ctx)
	}
}
