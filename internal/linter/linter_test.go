package linter

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

var tests = map[string][]Rule{
	"correct":                  allRules(),
	"invalid-include":          {&InvalidInclude{}},
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
	"duplicated-transaction":   {&DuplicatedTransaction{}},
	"duplicated-tag":           {&DuplicatedTag{}},
	"undeclared-tag":           {&UndeclaredTag{}},
	"undeclared-commodity":     {&UndeclaredCommodity{}},
	"undeclared-account":       {&UndeclaredAccount{}},
	"undeclared-payee":         {&UndeclaredPayee{}},
	"unbalanced-transaction":   {&UnbalancedTransaction{}},
	"unused-account":           {&UnusedAccount{}},
	"unused-tag":               {&UnusedTag{}},
	"invalid-date-tag":         {&InvalidDateTag{}},
	"invalid-type-tag":         {&InvalidTypeTag{}},
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
			rj, err := l.ResolveFS(fsys, "in.journal")
			if err != nil {
				t.Fatalf("failed to load test journal: %v", err)
			}

			ctx := analyzer.Build(rj)
			finds := (&Linter{rules: trules}).Run(ctx)

			// Rules must not set Severity themselves; it's assigned later by
			// the Reporter based on [Config].
			for _, f := range finds {
				if f.Severity != SeverityNone {
					t.Errorf("rule %s produced a Find with Severity = %v, want %v (unset)",
						f.Code, f.Severity, SeverityNone)
				}
			}

			var b strings.Builder
			fprint(&b, PathBasename, finds)
			golden.Assert(t, a, b.String())
		})
	}
}

func BenchmarkLinter(b *testing.B) {
	ldr := journal.NewLoader()
	rj, err := ldr.Resolve("../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	if err != nil {
		b.Fatalf("failed to load benchmark journal: %v", err)
	}

	ctx := analyzer.Build(rj)
	l, err := NewLinter(Config{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		l.Run(ctx)
	}
}

func allRules() []Rule {
	out := make([]Rule, 0, len(Rules))
	for _, b := range Rules {
		out = append(out, b.Rule)
	}
	return out
}

func TestNewLinter(t *testing.T) {
	t.Run("all-default", func(t *testing.T) {
		l, err := NewLinter(Config{})
		if err != nil {
			t.Fatal(err)
		}
		var got []RuleID
		for _, r := range l.rules {
			got = append(got, r.ID())
		}
		if !slices.Equal(got, builtinIDs()) {
			t.Errorf("got %v, want %v", got, builtinIDs())
		}
	})

	t.Run("disabled-omitted", func(t *testing.T) {
		l, err := NewLinter(Config{Rules: map[RuleID]RuleConfig{
			OrderDateID:    {Disabled: true},
			MissingPayeeID: {Disabled: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		var got []RuleID
		for _, r := range l.rules {
			got = append(got, r.ID())
		}
		want := without(builtinIDs(), OrderDateID, MissingPayeeID)
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("options-applied", func(t *testing.T) {
		l, err := NewLinter(Config{Rules: map[RuleID]RuleConfig{
			AccountDepthLimitID: {Options: json.RawMessage(`{"max-depth": 2}`)},
		}})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range l.rules {
			if r.ID() != AccountDepthLimitID {
				continue
			}
			if got := r.(*AccountDepthLimit).MaxDepth; got != 2 {
				t.Errorf("MaxDepth = %d, want 2", got)
			}
			if def := Rules[AccountDepthLimitID].Rule.(*AccountDepthLimit).MaxDepth; def != 4 {
				t.Errorf("shared built-in mutated: MaxDepth = %d, want 4", def)
			}
			return
		}
		t.Error("account-depth rule not found")
	})

	t.Run("options-on-non-optionable-rule", func(t *testing.T) {
		cfg := Config{Rules: map[RuleID]RuleConfig{
			ParseErrorID: {Options: json.RawMessage(`{"x":1}`)},
		}}
		if _, err := NewLinter(cfg); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("unknown-option-key", func(t *testing.T) {
		cfg := Config{Rules: map[RuleID]RuleConfig{
			AccountDepthLimitID: {Options: json.RawMessage(`{"nope": 1}`)},
		}}
		if _, err := NewLinter(cfg); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("malformed-options-json", func(t *testing.T) {
		cfg := Config{Rules: map[RuleID]RuleConfig{
			AccountDepthLimitID: {Options: json.RawMessage(`{not-json`)},
		}}
		if _, err := NewLinter(cfg); err == nil {
			t.Error("expected error for malformed JSON options")
		}
	})

	t.Run("rules-sorted-by-id", func(t *testing.T) {
		l, err := NewLinter(Config{})
		if err != nil {
			t.Fatal(err)
		}
		var got []RuleID
		for _, r := range l.rules {
			got = append(got, r.ID())
		}
		if !slices.IsSorted(got) {
			t.Errorf("rules are not sorted by ID: %v", got)
		}
	})
}

func TestConfig_SeverityFor(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfg := Config{}
		if got, want := cfg.SeverityFor(MissingPayeeID), Rules[MissingPayeeID].Severity; got != want {
			t.Errorf("SeverityFor() = %v, want %v", got, want)
		}
	})

	t.Run("override", func(t *testing.T) {
		cfg := Config{Rules: map[RuleID]RuleConfig{
			MissingPayeeID: {Severity: SeverityHint},
		}}
		if got := cfg.SeverityFor(MissingPayeeID); got != SeverityHint {
			t.Errorf("SeverityFor() = %v, want %v", got, SeverityHint)
		}
	})

	t.Run("zero-severity-falls-back-to-default", func(t *testing.T) {
		// A RuleConfig with Disabled=true and no explicit Severity leaves
		// Severity at its zero value (SeverityNone); SeverityFor must not
		// treat that as an override.
		cfg := Config{Rules: map[RuleID]RuleConfig{
			MissingPayeeID: {Disabled: true},
		}}
		if got, want := cfg.SeverityFor(MissingPayeeID), Rules[MissingPayeeID].Severity; got != want {
			t.Errorf("SeverityFor() = %v, want %v (default)", got, want)
		}
	})

	t.Run("unknown-rule", func(t *testing.T) {
		cfg := Config{}
		if got := cfg.SeverityFor("does-not-exist"); got != SeverityNone {
			t.Errorf("SeverityFor(unknown) = %v, want %v", got, SeverityNone)
		}
	})
}

func builtinIDs() []RuleID {
	ids := make([]RuleID, 0, len(Rules))
	for id := range Rules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func without(ids []RuleID, drop ...RuleID) []RuleID {
	out := ids[:0:0]
	for _, id := range ids {
		if !slices.Contains(drop, id) {
			out = append(out, id)
		}
	}
	return out
}
