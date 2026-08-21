package linter

import (
	"encoding/json"
	"testing"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal"
)

func TestAccountDepthLimit_Clone(t *testing.T) {
	orig := &AccountDepthLimit{MaxDepth: 5}
	cloned := orig.Clone()

	c, ok := cloned.(*AccountDepthLimit)
	if !ok {
		t.Fatalf("Clone() returned %T, want *AccountDepthLimit", cloned)
	}
	if c == orig {
		t.Fatal("Clone() returned the same pointer as the original")
	}
	if c.MaxDepth != 5 {
		t.Errorf("cloned MaxDepth = %d, want 5", c.MaxDepth)
	}

	c.MaxDepth = 99
	if orig.MaxDepth != 5 {
		t.Errorf("mutating the clone affected the original: MaxDepth = %d, want 5", orig.MaxDepth)
	}
}

func TestAccountDepthLimit_UnmarshalOptions(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		a := &AccountDepthLimit{MaxDepth: 4}
		if err := a.UnmarshalOptions(json.RawMessage(`{"max-depth": 2}`)); err != nil {
			t.Fatalf("UnmarshalOptions() error = %v", err)
		}
		if a.MaxDepth != 2 {
			t.Errorf("MaxDepth = %d, want 2", a.MaxDepth)
		}
	})

	t.Run("empty-object-keeps-default", func(t *testing.T) {
		a := &AccountDepthLimit{MaxDepth: 4}
		if err := a.UnmarshalOptions(json.RawMessage(`{}`)); err != nil {
			t.Fatalf("UnmarshalOptions() error = %v", err)
		}
		if a.MaxDepth != 4 {
			t.Errorf("MaxDepth = %d, want 4 (unchanged)", a.MaxDepth)
		}
	})

	t.Run("unknown-field", func(t *testing.T) {
		a := &AccountDepthLimit{}
		if err := a.UnmarshalOptions(json.RawMessage(`{"nope": 1}`)); err == nil {
			t.Error("UnmarshalOptions() error = nil, want error for unknown field")
		}
	})

	t.Run("malformed-json", func(t *testing.T) {
		a := &AccountDepthLimit{}
		if err := a.UnmarshalOptions(json.RawMessage(`{not-json`)); err == nil {
			t.Error("UnmarshalOptions() error = nil, want error for malformed JSON")
		}
	})

	t.Run("wrong-value-type", func(t *testing.T) {
		a := &AccountDepthLimit{}
		if err := a.UnmarshalOptions(json.RawMessage(`{"max-depth": "two"}`)); err == nil {
			t.Error("UnmarshalOptions() error = nil, want error for wrong value type")
		}
	})
}

func TestAccountDepthLimit_CheckJournal(t *testing.T) {
	src := []byte(`2024-01-01 * "test"
    assets:cash:sub:deep  10.00 USD
    assets:cash
`)
	rj := journal.NewLoader().ResolveBytes("in.journal", src)
	an := analyzer.Build(rj)

	rule := &AccountDepthLimit{MaxDepth: 1}
	finds := rule.CheckJournal(an)

	if len(finds) == 0 {
		t.Fatal("CheckJournal() returned no finds, want at least one for accounts exceeding MaxDepth")
	}
	for _, f := range finds {
		if f.Code != AccountDepthLimitID {
			t.Errorf("Code = %q, want %q", f.Code, AccountDepthLimitID)
		}
		// Severity is no longer set by the rule itself; it's assigned later
		// by the Reporter based on Config.
		if f.Severity != SeverityNone {
			t.Errorf("Severity = %v, want %v (unset)", f.Severity, SeverityNone)
		}
		if f.Message == "" {
			t.Error("Message is empty")
		}
	}
}

func TestAccountDepthLimit_CheckJournal_WithinLimit(t *testing.T) {
	src := []byte(`2024-01-01 * "test"
    assets:cash  10.00 USD
    expenses:food
`)
	rj := journal.NewLoader().ResolveBytes("in.journal", src)
	an := analyzer.Build(rj)

	rule := &AccountDepthLimit{MaxDepth: 4}
	if finds := rule.CheckJournal(an); len(finds) != 0 {
		t.Errorf("CheckJournal() = %v, want no finds for accounts within MaxDepth", finds)
	}
}