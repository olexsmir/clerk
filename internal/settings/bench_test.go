package settings

import (
	"testing"

	"olexsmir.xyz/clerk/internal/linter"
)

var benchTOML = []byte(`
semantic_highlighting = true
latin_to_cyrillic_completion = false
format = { tab-indent = true, indent-width = 4, align-style = "right", commodity-pos = "before", align-column = 80, preserve-blank-lines = true }
[lint]
unbalanced-transaction = "error"
missing-payee = false
empty-postings = "warn"
account-depth = { severity = "error", max-depth = 8 }
`)

var benchRawMap = map[string]any{
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

func BenchmarkParseTOML(b *testing.B) {
	for b.Loop() {
		if _, _, err := ParseTOML(benchTOML); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMap(b *testing.B) {
	for b.Loop() {
		if _, _, err := Parse(benchRawMap); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLargeLint(b *testing.B) {
	raw := map[string]any{"lint": map[string]any{}}
	for id := range linter.Rules {
		raw["lint"].(map[string]any)[string(id)] = "error"
	}
	for b.Loop() {
		if _, _, err := Parse(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormKey(b *testing.B)          { benchNormKey(b, "latinToCyrillicCompletion") }
func BenchmarkNormKeyCanonical(b *testing.B) { benchNormKey(b, "semantic_highlighting") }
func BenchmarkNormKeyKebab(b *testing.B)     { benchNormKey(b, "unbalanced-transaction") }
func benchNormKey(b *testing.B, s string) {
	for i := 0; i < b.N; i++ {
		_ = normalizeKey(s)
	}
}
