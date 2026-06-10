package printer

import (
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

var tests = []string{"entries", "directives", "sample"}

func TestRoundTrip(t *testing.T) {
	for _, tname := range tests {
		t.Run(tname, func(t *testing.T) {
			inp := golden.Load(t, tname)

			pf, err := journal.NewLoader().LoadBytes(tname+".journal", inp)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			if err := defaultConfig.Fprint(&b, pf.Ast); err != nil {
				t.Fatal(err)
			}

			golden.Assert(t, tname, b.String())
		})
	}
}

func BenchmarkPrinter(b *testing.B) {
	for _, tname := range tests {
		b.Run(tname, func(b *testing.B) {
			b.ReportAllocs()
			inp := golden.Load(b, tname)
			pf, err := journal.NewLoader().LoadBytes(tname+".journal", inp)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var buf strings.Builder
				if err := defaultConfig.Fprint(&buf, pf.Ast); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

var testsWithConfig = map[string]*Config{
	"align_right":      {AlignStyle: AlignRight, AlignColumn: 50},
	"align_tab":        {AlignStyle: AlignTab, AlignColumn: 50},
	"commodity_before": {CommodityPos: CommodityBefore},
	"preserve_blanks":  {PreserveBlankLines: true},
	"tab_indent":       {TabIndent: true},
	"indent_width":     {IndentWidth: 4},
}

func TestRoundTrip_WithConfig(t *testing.T) {
	for tname, tt := range testsWithConfig {
		t.Run(tname, func(t *testing.T) {
			inp := golden.Load(t, tname)
			pf, err := journal.NewLoader().LoadBytes(tname+".journal", inp)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			if err := tt.Fprint(&b, pf.Ast); err != nil {
				t.Fatal(err)
			}

			golden.Assert(t, tname, b.String())
		})
	}
}

func BenchmarkPrinter_Config(b *testing.B) {
	inp := golden.Load(b, "entries")
	pf, err := journal.NewLoader().LoadBytes("entries.journal", inp)
	if err != nil {
		b.Fatal(err)
	}
	for name, cfg := range testsWithConfig {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var buf strings.Builder
				if err := cfg.Fprint(&buf, pf.Ast); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
