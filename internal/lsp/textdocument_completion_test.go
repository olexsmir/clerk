// TODO:
package lsp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestDetectCompletion_NoJournal(t *testing.T) {
	// detection must not depend on a parse
	i := strings.Index("    no^te foo\n", "^")
	content := "    note foo\n"
	ctx, _ := detectCompletionCtx(content, i)
	if ctx != cmplAccount {
		t.Errorf("ctx = %v, want account", ctx)
	}
}

func TestDateStyle(t *testing.T) {
	tests := map[string]struct {
		history []string
		sep     byte
		hasYear bool
	}{
		"empty":    {nil, '-', true},
		"dash":     {[]string{"2024-01-02"}, '-', true},
		"slash":    {[]string{"2024/01/20"}, '/', true},
		"yearless": {[]string{"6-10"}, '-', false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sep, hasYear := dateStyle(tt.history)
			if sep != tt.sep || hasYear != tt.hasYear {
				t.Errorf("dateStyle = %q/%v, want %q/%v", sep, hasYear, tt.sep, tt.hasYear)
			}
		})
	}
}

func TestDateTokenEnd(t *testing.T) {
	tests := map[string]struct {
		content string
		start   int
		want    int
	}{
		"full date":            {"2024-01-02 x", 0, 10},
		"unpadded":             {"2024/1/2 x", 0, 8},
		"yearless":             {"6-10 x", 0, 4},
		"stops at =":           {"2024-01-02=2024-01-03", 0, 10},
		"stops at comment":     {"2024-01-02;x", 0, 10},
		"partial trailing sep": {"2024- x", 0, 5},
		"partial mid":          {"2024-0 x", 0, 6},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := dateTokenEnd(tt.content, tt.start); got != tt.want {
				t.Errorf("dateTokenEnd = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDatePatternMatch(t *testing.T) {
	tests := map[string]struct {
		pattern, canonical string
		want               bool
	}{
		"digit subsequence":   {"2024-01-0", "2024-01-05", true},
		"same sep":            {"2024/0", "2024/01/20", true},
		"other sep rejected":  {"2024/0", "2024-01-20", false},
		"yearless":            {"6-10", "6-10", true},
		"yearless padded rej": {"06-10", "6-10", false},
		"no match":            {"2025", "2024-01-20", false},
		"empty":               {"", "2024-01-20", true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := datePatternMatch(tt.pattern, tt.canonical); got != tt.want {
				t.Errorf("datePatternMatch = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGolden_Completion(t *testing.T) {
	for _, tt := range []string{"completion-contexts", "completion-journal", "completion-no-transactions"} {
		t.Run(tt, func(t *testing.T) {
			ar := golden.Read(t, tt)
			h := newTxtarHarness(t, ar)
			if err := h.srv.applySettings(t.Context(), []byte(`{"latin_to_cyrillic_completion": true}`)); err != nil {
				t.Fatal(err)
			}

			var b strings.Builder
			for i, c := range h.cursors {
				tdp := h.textDocumentPosition(i)
				res, err := h.srv.Completion(t.Context(), &protocol.CompletionParams{
					TextDocumentPositionParams: tdp,
				})
				if err != nil {
					t.Fatal(err)
				}
				ctx, start := detectCompletionCtx(h.content, c)
				fmt.Fprintf(&b, "%d:%d %s %q\n", tdp.Position.Line, tdp.Position.Character, ctx, h.content[start:c])
				list, ok := res.(*protocol.CompletionList)
				if !ok {
					t.Fatalf("Completion returned %T, want *protocol.CompletionList", res)
				}
				now := time.Now()
				var sep byte
				var hasYear bool
				if ctx == cmplDate {
					if an := h.srv.analysisFor(h.uri); an != nil {
						sep, hasYear = dateStyle(an.DateStrings)
					}
				}
				for _, item := range list.Items {
					label := item.Label
					if ctx == cmplDate {
						switch label {
						case renderDate(now, sep, hasYear):
							label = "today"
						case renderDate(now.AddDate(0, 0, -1), sep, hasYear):
							label = "yesterday"
						case renderDate(now.AddDate(0, 0, -2), sep, hasYear):
							label = "2 days ago"
						}
					}
					fmt.Fprintf(&b, "  %s\n", label)
				}
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

func (c cmplCtx) String() string {
	switch c {
	case cmplNone:
		return "none"
	case cmplAccount:
		return "account"
	case cmplPayee:
		return "payee"
	case cmplCommodity:
		return "commodity"
	case cmplTagName:
		return "tag"
	case cmplTagValue:
		return "tag-value"
	case cmplDate:
		return "date"
	case cmplDirective:
		return "directive"
	default:
		return "?"
	}
}

func BenchmarkCompletion(b *testing.B) {
	path := "../../journal/testdata/journals/actual-1ktxns-100accts.journal"
	rj, err := journal.NewLoader().Resolve(path)
	if err != nil {
		b.Fatal(err)
	}
	content := string(rj.Occurrences[0].Src)

	srv := newServer(b)
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
	srv.server.analysisFor(uri.URI("file:///test.journal")) // warm the per-doc cache

	for tname, tt := range map[string]int{
		"1k txns, account":        strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2,
		"1k txns, empty payee":    strings.Index(content, "transaction 1") + len("transaction "),
		"1k txns, commodity":      strings.Index(content, "2 B @@") + len("2 B"),
		"1k txns, date":           strings.Index(content, "2000-01-0") + len("2000-01-0"),
		"1k txns, price date":     strings.Index(content, "P 2000-01-0") + len("P 2000-01-0"),
		"1k txns, price quantity": strings.Index(content, "P 2000-01-01 A  0.70") + len("P 2000-01-01 A  0.70"),
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt)
			params := &protocol.CompletionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
				Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
			}

			// warm up
			if _, err := srv.server.Completion(b.Context(), params); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.Completion(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}
			// guard: line-local lexing must stay far below the old whole-file
			// relex (~3ms on this file); the measured target is sub-ms
			if avg := b.Elapsed() / time.Duration(b.N); avg > 2*time.Millisecond {
				b.Fatalf("completion %v/op: whole-file relex regression", avg)
			}
		})
	}
}

func BenchmarkCompletionTransliteration(b *testing.B) {
	tails := []string{"а", "б", "в", "г", "д", "е", "є", "ж", "з", "и", "і", "ї", "й", "к", "л", "м", "н", "о", "п", "р"}
	var sb strings.Builder
	for _, n := range []string{"Витрати", "Доходи", "Активи", "Капітал", "Зобовязання"} {
		for _, t := range tails {
			fmt.Fprintf(&sb, "account %s:%s%s\n", n, n, t)
		}
	}
	sb.WriteString("account vyt")
	content := sb.String()

	srv := newServer(b)
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
	srv.server.analysisFor(uri.URI("file:///test.journal")) // warm the per-doc cache
	if err := srv.server.applySettings(b.Context(), []byte(`{"latin_to_cyrillic_completion": true}`)); err != nil {
		b.Fatal(err)
	}

	line, col := lsputil.LineCol(content, len(content))
	params := &protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
		Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
	}

	warm, err := srv.server.Completion(b.Context(), params)
	if err != nil {
		b.Fatal(err)
	}

	// Guard: the Cyrillic corpus completes only through the transliterated
	// pattern; without it the list collapses to the latin "vyt" placeholder.
	if got := len(warm.(*protocol.CompletionList).Items); got <= 1 {
		b.Fatalf("transliteration matching did not engage (items: %d)", got)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := srv.server.Completion(b.Context(), params); err != nil {
			b.Fatal(err)
		}
	}
}
