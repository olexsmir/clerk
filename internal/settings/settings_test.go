package settings

import (
	"reflect"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal/printer"
)

func TestNormKey(t *testing.T) {
	cases := map[string]string{
		"semantic_highlighting":        "semantic_highlighting",
		"semantic-highlighting":        "semantic_highlighting",
		"semanticHighlighting":         "semantic_highlighting",
		"latin_to_cyrillic_completion": "latin_to_cyrillic_completion",
		"latinToCyrillicCompletion":    "latin_to_cyrillic_completion",
		"format":                       "format",
		"ALIGN_STYLE":                  "align_style",
		"indent-width":                 "indent_width",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Errorf("normKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseClonesDefaults(t *testing.T) {
	base := DefaultConfig
	if _, _, err := Parse(map[string]any{
		"format": map[string]any{"indent-width": int64(6)},
		"lint":   map[string]any{"missing-payee": "error"},
	}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(DefaultConfig, base) {
		t.Error("Parse mutated the global DefaultConfig")
	}
	s, _, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if !reflect.DeepEqual(s, base) {
		t.Error("Parse(nil) leaked state from an earlier Parse")
	}
}

func TestParseAppliesFormat(t *testing.T) {
	s, _, err := Parse(map[string]any{
		"format": map[string]any{
			"tab-indent":    true,
			"indent-width":  int64(4),
			"align-style":   "right",
			"commodity-pos": "before",
			"align-column":  int64(80),
		},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !s.Format.TabIndent || s.Format.IndentWidth != 4 ||
		s.Format.AlignStyle != printer.AlignRight ||
		s.Format.CommodityPos != printer.CommodityBefore ||
		s.Format.AlignColumn != 80 {
		t.Errorf("format not applied: %+v", s.Format)
	}
}

func TestParseAppliesLint(t *testing.T) {
	s, _, err := Parse(map[string]any{
		"lint": map[string]any{
			"unbalanced-transaction": "info",
			"missing-payee":          false,
			"empty-postings":         "warn",
			"account-depth": map[string]any{
				"severity":  "error",
				"max-depth": int64(8),
			},
		},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if rc := s.Linter.Rules[linter.UnbalancedTransactionID]; rc.Severity != linter.SeverityInfo {
		t.Errorf("unbalanced-transaction severity = %v, want info", rc.Severity)
	}
	if rc := s.Linter.Rules[linter.MissingPayeeID]; !rc.Disabled {
		t.Error("missing-payee should be disabled")
	}
	if rc := s.Linter.Rules[linter.EmptyPostingsID]; rc.Severity != linter.SeverityWarning {
		t.Errorf("empty-postings severity = %v, want warn", rc.Severity)
	}
	if rc := s.Linter.Rules[linter.AccountDepthLimitID]; rc.Severity != linter.SeverityError {
		t.Errorf("account-depth severity = %v, want error", rc.Severity)
	}
	if rc := s.Linter.Rules[linter.AccountDepthLimitID]; string(rc.Options) != `{"max-depth":8}` {
		t.Errorf("account-depth options = %s, want %q", rc.Options, `{"max-depth":8}`)
	}
	// Options must not include the severity key: the rule's UnmarshalOptions
	// rejects unknown fields, so a valid config proves severity was stripped.
	if _, err := linter.NewLinter(s.Linter); err != nil {
		t.Fatalf("config should validate: %v", err)
	}
}

func TestApplyLSP(t *testing.T) {
	s := DefaultConfig
	_, err := s.ApplyLSP(map[string]any{
		"semanticHighlighting":      false,
		"latinToCyrillicCompletion": true,
		"format":                    map[string]any{"indent-width": int64(4)},
	})
	if err != nil {
		t.Fatalf("ApplyLSP: %v", err)
	}
	if s.SemanticHighlighting {
		t.Error("SemanticHighlighting = true, want false")
	}
	if !s.LatinToCyrillicCompletion {
		t.Error("LatinToCyrillicCompletion = false, want true")
	}
	if s.Format.IndentWidth != 4 {
		t.Errorf("IndentWidth = %d, want 4", s.Format.IndentWidth)
	}
}

func TestUnknownKeysWarn(t *testing.T) {
	s := DefaultConfig
	warns, err := s.Apply(map[string]any{
		"unknown_top": "x",
		"format":      map[string]any{"bogus": true},
		"lint":        map[string]any{"nope-rule": "error"},
	})
	if err != nil {
		t.Fatalf("unknown keys should warn, not error: %v", err)
	}
	if len(warns) != 3 {
		t.Fatalf("got %d warnings, want 3: %v", len(warns), warns)
	}
	for _, w := range warns {
		if !strings.Contains(w, "unknown") {
			t.Errorf("warning %q: want unknown-key message", w)
		}
	}
}

func TestApplyErrors(t *testing.T) {
	s := DefaultConfig

	err := func(raw map[string]any) error {
		_, err := s.Apply(raw)
		return err
	}
	if err(map[string]any{"format": map[string]any{"indent-width": 2.5}}) == nil {
		t.Error("expected error for non-integral float indent-width")
	}
	for _, n := range []any{0, 17} {
		if err(map[string]any{"format": map[string]any{"indent-width": n}}) == nil {
			t.Errorf("indent-width %v: expected error", n)
		}
	}
	for _, n := range []any{0, 241} {
		if err(map[string]any{"format": map[string]any{"align-column": n}}) == nil {
			t.Errorf("align-column %v: expected error", n)
		}
	}
	if err(map[string]any{"lint": map[string]any{"unbalanced-transaction": true}}) == nil {
		t.Error("expected error for lint rule set to true")
	}
	if err(map[string]any{"format": map[string]any{"align-style": "none"}}) == nil {
		t.Error("expected error for unknown align-style")
	}

	if err(map[string]any{"format": map[string]any{"indent-width": 4.0}}) != nil {
		t.Error("integral float should be accepted")
	}
}

func TestParseTOML(t *testing.T) {
	if s, _, err := ParseTOML([]byte(`format = { indent-width = 4 }`)); err != nil {
		t.Fatalf("ParseTOML: %v", err)
	} else if s.Format.IndentWidth != 4 {
		t.Errorf("IndentWidth = %d, want 4", s.Format.IndentWidth)
	}
	if _, _, err := ParseTOML([]byte(`42`)); err == nil {
		t.Error("expected error for non-table TOML")
	}
}
