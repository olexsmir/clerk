package fuzzy

import (
	"math"
	"testing"
)

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		want    float64 // -1 means: expect 0 (no match)
	}{
		{"empty pattern matches everything", "", "expenses:food", 1},
		{"exact boundary match", "food", "expenses:food", 1},
		{"long exact match also perfect", "expenses", "expenses:food", 1},
		{"acronym across segments", "expf", "expenses:food", 1},
		{"colon need not be typed", "expensesfood", "expenses:food", 1},
		{"all-caps pattern still perfect", "FOOD", "expenses:food", 1},
		{"gapped mid-word match", "od", "expenses:food", 0.444},
		{"boundary beats mid-word", "xf", "expenses:food", 0.778},
		{"exact prefix", "ex", "expenses", 1},
		{"gap penalty", "esf", "expenses:food", 0.923},
		{"no match", "xyz", "abc", 0},
		{"cjk perfect match", "食物", "支出:食物", 1},
		{"ukrainian perfect match", "продукти", "витрати:продукти", 1},
		{"cjk gap", "食物", "支出费物", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.pattern, tt.text)
			if tt.want == 0 {
				if got != 0 {
					t.Errorf("Score(%q, %q) = %v, want 0", tt.pattern, tt.text, got)
				}
				return
			}
			if math.Abs(got-tt.want) > 1e-3 {
				t.Errorf("Score(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
			}
		})
	}
}

func TestFuzzyScoreDeterministic(t *testing.T) {
	for _, text := range []string{"expenses:food", "支出:食物", "витрати:продукти"} {
		for _, pattern := range []string{"", "ex", "expf", "o", "食物", "п"} {
			if a, b := Score(pattern, text), Score(pattern, text); a != b {
				t.Errorf("Score(%q, %q) not deterministic: %v != %v", pattern, text, a, b)
			}
		}
	}
}
