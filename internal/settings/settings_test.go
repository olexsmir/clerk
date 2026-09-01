package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/internal/testutil/golden"
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

// Benchmarks

func BenchmarkParseMap(b *testing.B) {
	benchRawMap := map[string]any{
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
			"account-depth":          map[string]any{"severity": "error", "max-depth": int64(8)},
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := parse(benchRawMap); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLargeLint(b *testing.B) {
	raw := map[string]any{"lint": map[string]any{}}
	for id := range linter.Rules {
		raw["lint"].(map[string]any)[string(id)] = "error"
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := parse(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormKey(b *testing.B) {
	tests := []string{"latinToCyrillicCompletion", "semantic_highlighting", "unbalanced-transaction"}
	for _, test := range tests {
		b.ReportAllocs()
		b.Run(test, func(b *testing.B) {
			for b.Loop() {
				_ = normalizeKey(test)
			}
		})
	}
}

// Golden tests

func TestGolden_Load(t *testing.T) {
	for _, tt := range []string{"different-cases", "full", "invalid-values", "missing", "non-table", "unknown-keys"} {
		t.Run(tt, func(t *testing.T) {
			ar := golden.Read(t, tt)
			path := filepath.Join(t.TempDir(), "clerk.toml")
			if cfg := ar.Get("config.toml"); cfg != nil {
				if err := os.WriteFile(path, cfg, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			s, warns, err := Load(path)
			golden.Assert(t, ar, renderTOML(s, warns, err))
		})
	}
}

func renderTOML(s Settings, warns []string, err error) string {
	var lines []string
	for _, w := range warns {
		lines = append(lines, "warning: "+w)
	}
	if err != nil {
		for line := range strings.SplitSeq(err.Error(), "\n") {
			if line != "" {
				lines = append(lines, "error: "+line)
			}
		}
	}
	slices.Sort(lines)
	if err == nil {
		lines = append(lines, changedLines(configLines(s), configLines(DefaultConfig))...)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func configLines(s Settings) []string {
	var lines []string
	for _, line := range renderFormat(s.Format) {
		lines = append(lines, "format."+line)
	}
	for _, id := range sortedRuleIDs() {
		rc := s.Linter.Rules[id]
		line := "lint." + string(id) + ": "
		if rc.Disabled {
			line += "disabled"
		} else {
			line += rc.Severity.String()
			if len(rc.Options) > 0 {
				line += " options=" + string(rc.Options)
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func changedLines(got, want []string) []string {
	var out []string
	for i, line := range got {
		if line != want[i] {
			out = append(out, line)
		}
	}
	return out
}

func renderFormat(c printer.Config) []string {
	return []string{
		"tab-indent: " + fmt.Sprint(c.TabIndent),
		"indent-width: " + fmt.Sprint(c.IndentWidth),
		"preserve-blank-lines: " + fmt.Sprint(c.PreserveBlankLines),
		"align-style: " + c.AlignStyle.String(),
		"align-column: " + fmt.Sprint(c.AlignColumn),
		"commodity-pos: " + c.CommodityPos.String(),
	}
}

func sortedRuleIDs() []linter.RuleID {
	ids := make([]linter.RuleID, 0, len(linter.Rules))
	for id := range linter.Rules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
