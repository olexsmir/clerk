package settings

import (
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
		if got := normKey(in); got != want {
			t.Errorf("normKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTopLevel(t *testing.T) {
	s, warns, err := Parse(map[string]any{
		"format": map[string]any{
			"tab-indent":    true,
			"indent-width":  int64(4),
			"align-style":   "right",
			"commodity-pos": "before",
		},
		"lint": map[string]any{
			"unbalanced-transaction": "info",
			"missing-payee":          false,
		},
		"unknown_top": "x",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "unknown_top") {
		t.Fatalf("expected one unknown-top warning, got %v", warns)
	}
	if !s.Format.TabIndent || s.Format.IndentWidth != 4 {
		t.Errorf("format not applied: %+v", s.Format)
	}
	if s.Format.AlignStyle != printer.AlignRight {
		t.Errorf("AlignStyle = %v, want AlignRight", s.Format.AlignStyle)
	}
	if s.Format.CommodityPos != printer.CommodityBefore {
		t.Errorf("CommodityPos = %v, want CommodityBefore", s.Format.CommodityPos)
	}
	if rc := s.Linter.Rules[linter.UnbalancedTransactionID]; rc.Severity != linter.SeverityInfo {
		t.Errorf("unbalanced-transaction severity = %v, want info", rc.Severity)
	}
	if rc := s.Linter.Rules[linter.MissingPayeeID]; !rc.Disabled {
		t.Errorf("missing-payee should be disabled")
	}
}

func TestParseRejectsLSPOnly(t *testing.T) {
	s, warns, err := Parse(map[string]any{
		"semanticHighlighting":      false,
		"latinToCyrillicCompletion": true,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !s.SemanticHighlighting {
		t.Errorf("SemanticHighlighting = false, want default true (file value ignored)")
	}
	if s.LatinToCyrillicCompletion {
		t.Errorf("LatinToCyrillicCompletion = true, want default false (file value ignored)")
	}
	if len(warns) != 2 {
		t.Fatalf("expected 2 unknown-setting warnings, got %v", warns)
	}
	for _, w := range warns {
		if !strings.Contains(w, "unknown setting") {
			t.Errorf("warning %q: want generic unknown-setting message, not LSP-specific", w)
		}
	}
}

func TestApplyLSP(t *testing.T) {
	s := Default()
	_, err := s.ApplyLSP(map[string]any{
		"semanticHighlighting":      false,
		"latinToCyrillicCompletion": true,
		"format":                    map[string]any{"indent-width": int64(4)},
	})
	if err != nil {
		t.Fatalf("ApplyLSP returned error: %v", err)
	}
	if s.SemanticHighlighting {
		t.Errorf("SemanticHighlighting = true, want false")
	}
	if !s.LatinToCyrillicCompletion {
		t.Errorf("LatinToCyrillicCompletion = false, want true")
	}
	if s.Format.IndentWidth != 4 {
		t.Errorf("IndentWidth = %d, want 4", s.Format.IndentWidth)
	}
}

func TestApplyUnknownFormatWarns(t *testing.T) {
	s := Default()
	warns, err := s.Apply(map[string]any{
		"format": map[string]any{"bogus": true},
	})
	if err != nil {
		t.Fatalf("expected no error for unknown format option, got %v", err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "bogus") {
		t.Fatalf("expected unknown format warning, got %v", warns)
	}
}

func TestApplyUnknownLintWarns(t *testing.T) {
	s := Default()
	warns, err := s.Apply(map[string]any{
		"lint": map[string]any{"nope-rule": "error"},
	})
	if err != nil {
		t.Fatalf("expected no error for unknown lint rule, got %v", err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "nope-rule") {
		t.Fatalf("expected unknown lint warning, got %v", warns)
	}
}

func TestApplyFloatRejected(t *testing.T) {
	s := Default()
	if _, err := s.Apply(map[string]any{
		"format": map[string]any{"indent-width": 2.5},
	}); err == nil {
		t.Fatal("expected error for non-integral float indent-width")
	}
}

func TestApplyIntegralFloatAccepted(t *testing.T) {
	s := Default()
	if _, err := s.Apply(map[string]any{
		"format": map[string]any{"indent-width": 4.0},
	}); err != nil {
		t.Fatalf("integral float should be accepted: %v", err)
	}
	if s.Format.IndentWidth != 4 {
		t.Errorf("IndentWidth = %d, want 4", s.Format.IndentWidth)
	}
}

func TestApplyBounds(t *testing.T) {
	s := Default()
	for _, n := range []any{0, 17} {
		if _, err := s.Apply(map[string]any{"format": map[string]any{"indent-width": n}}); err == nil {
			t.Errorf("indent-width %v: expected error", n)
		}
	}
	for _, n := range []any{0, 241} {
		if _, err := s.Apply(map[string]any{"format": map[string]any{"align-column": n}}); err == nil {
			t.Errorf("align-column %v: expected error", n)
		}
	}
}

func TestApplyLintTrueRejected(t *testing.T) {
	s := Default()
	if _, err := s.Apply(map[string]any{
		"lint": map[string]any{"unbalanced-transaction": true},
	}); err == nil {
		t.Fatal("expected error for lint rule set to true")
	}
}

func TestDefaultFormat(t *testing.T) {
	if Default().Format != *printer.DefaultConfig {
		t.Errorf("Default().Format = %+v, want %+v", Default().Format, *printer.DefaultConfig)
	}
}

func TestParseTOMLNonTable(t *testing.T) {
	if _, _, err := ParseTOML([]byte("42")); err == nil {
		t.Error("expected error for non-table TOML")
	}
}

var benchTOML = []byte(`
semantic_highlighting = true
latin_to_cyrillic_completion = false
format = { tab-indent = true, indent-width = 4, align-style = "right", commodity-pos = "before", align-column = 80, preserve-blank-lines = true }
[lint]
unbalanced-transaction = "error"
missing-payee = false
empty-postings = "warn"
`)

func benchRaw() map[string]any {
	return map[string]any{
		"semantic_highlighting": true,
		"format": map[string]any{
			"tab-indent":           true,
			"indent-width":         int64(4),
			"align-style":          "right",
			"commodity-pos":        "before",
			"align-column":         int64(80),
			"preserve-blank-lines": true,
		},
		"lint": map[string]any{
			"unbalanced-transaction": "error",
			"missing-payee":          false,
			"empty-postings":         "warn",
		},
	}
}

func BenchmarkParseTOML(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := ParseTOML(benchTOML); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMap(b *testing.B) {
	raw := benchRaw()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := Parse(raw); err != nil {
			b.Fatal(err)
		}
	}
}
