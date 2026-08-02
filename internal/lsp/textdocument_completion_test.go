package lsp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestDetectCompletionCtx(t *testing.T) {
	tests := []struct {
		name string
		ctx  cmplCtx
		in   string
		want string
	}{
		{"posting account", cmplAccount, "    expenses:f^ood  $50\n", "expenses:f"},
		{"posting account empty", cmplAccount, "    ^\n", ""},
		{"posting account after colon", cmplAccount, "    expenses:^food  $50\n", "expenses:"},
		{"posting amount commodity", cmplCommodity, "    expenses:food  $^50\n", "$"},
		{"posting empty amount region", cmplCommodity, "    expenses:food  ^\n", ""},
		{"posting commodity word", cmplCommodity, "    expenses:food  U^SD\n", "U"},
		{"posting amount number", cmplNone, "    expenses:food  $5^0\n", ""},
		{"posting cost quantity", cmplNone, "    expenses:food  $50 @^ 1.5\n", ""},
		{"posting status", cmplAccount, "    * expenses:f^ood  $50\n", "expenses:f"},
		{"posting virtual", cmplAccount, "    (expenses:f^ood)  $50\n", "expenses:f"},
		{"posting comment tag", cmplTagName, "    expenses:food  ; clie^nt:x\n", "clie"},
		{"posting comment tag value", cmplNone, "    expenses:food  ; client:^x\n", ""},
		{"header payee", cmplPayee, "2024-01-15 acm^e\n    assets:cash\n", "acm"},
		{"header payee empty", cmplPayee, "2024-01-15 ^\n", ""},
		{"header payee right after date", cmplPayee, "2024-01-15^\n", ""},
		{"header status and code", cmplPayee, "2024-01-15 * (123) gro^cer\n", "gro"},
		{"header second date", cmplPayee, "2024-01-15=2024-01-16 acm^e\n", "acm"},
		{"header quoted payee", cmplPayee, "2024-01-15 \"ac^me\"\n", "ac"},
		{"header pipe note", cmplNone, "2024-01-15 acme | no^te\n", ""},
		{"header pipe inline", cmplNone, "2024-01-15 acme|note x^y\n", ""},
		{"header inline comment", cmplTagName, "2024-01-15 ; foo^", "foo"},
		{"directive keyword partial", cmplDirective, "acc^ount expenses\n", "acc"},
		{"directive keyword empty line", cmplDirective, "\n^", ""},
		{"account directive value", cmplAccount, "account exp^enses\n", "exp"},
		{"commodity directive value", cmplCommodity, "commodity U^SD\n", "U"},
		{"payee directive value", cmplPayee, "payee ac^me\n", "ac"},
		{"tag directive value", cmplTagName, "tag pro^ject\n", "pro"},
		{"comment tag", cmplTagName, "; clie^nt:x\n", "clie"},
		{"comment value", cmplNone, "; client:x^yz\n", ""},
		{"comment plain text", cmplTagName, "; groc^eries\n", "groc"},
		{"subdirective ignored", cmplNone, "account expenses\n    no^te ignore\n", ""},
		{"periodic header", cmplNone, "~ monthly^ budget\n", ""},
		{"cjk posting", cmplAccount, "    支出:食^物  50\n", "支出:食"},
		{"ukrainian payee", cmplPayee, "2024-01-15 прод^укти\n", "прод"},
		{"crlf posting", cmplAccount, "2024-01-15 x\r\n    expenses:f^ood  $50\r\n", "expenses:f"},
		{"crlf header", cmplPayee, "2024-01-15 acm^e\r\n    assets:cash\r\n", "acm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cont := tt.in
			i := strings.Index(cont, "^")
			if i < 0 {
				t.Fatal("no cursor marker '^' in content")
			}

			cont = cont[:i] + cont[i+1:]
			ctx, start := detectCompletionCtx(cont, i)
			prefix := cont[start:i]

			if ctx != tt.ctx {
				t.Errorf("ctx = %v, want %v", ctx, tt.ctx)
			}
			if prefix != tt.want {
				t.Errorf("prefix = %q, want %q", prefix, tt.want)
			}
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
	case cmplDirective:
		return "directive"
	default:
		return "?"
	}
}

func TestDetectCompletion_NoJournal(t *testing.T) {
	// detection must not depend on a parse
	i := strings.Index("    no^te foo\n", "^")
	content := "    note foo\n"
	ctx, _ := detectCompletionCtx(content, i)
	if ctx != cmplAccount {
		t.Errorf("ctx = %v, want account", ctx)
	}
}

func TestDetectCompletion_Subdirective(t *testing.T) {
	// the lexical rule suppresses completion on subdirective lines without a parse
	i := strings.Index("account expenses\n    no^te ignore\n", "^")
	content := "account expenses\n    note ignore\n"
	if ctx, _ := detectCompletionCtx(content, i); ctx != cmplNone {
		t.Errorf("subdirective: ctx = %v, want none", ctx)
	}
	// a posting line after a transaction header is still an account context
	i = strings.Index("2024-01-15 acme\n    expe^nses:food  $50\n", "^")
	content = "2024-01-15 acme\n    expenses:food  $50\n"
	if ctx, _ := detectCompletionCtx(content, i); ctx != cmplAccount {
		t.Errorf("posting: ctx = %v, want account", ctx)
	}
	// a blank line ends the directive body; a whitespace-only line does not
	i = strings.Index("account expenses\n    note: x\n\n    no^te\n", "^")
	content = "account expenses\n    note: x\n\n    note\n"
	if ctx, _ := detectCompletionCtx(content, i); ctx != cmplAccount {
		t.Errorf("after blank line: ctx = %v, want account", ctx)
	}
	i = strings.Index("account expenses\n    note: x\n   \n    no^te\n", "^")
	content = "account expenses\n    note: x\n   \n    note\n"
	if ctx, _ := detectCompletionCtx(content, i); ctx != cmplNone {
		t.Errorf("whitespace-only line keeps body: ctx = %v, want none", ctx)
	}
}

func TestCompleteItems_NoTransactions(t *testing.T) {
	// directives-only doc: empty a.Dates must not panic the ranking
	content := "account expenses:food\n\n^"
	i := strings.Index(content, "^")
	content = content[:i]
	a := analyzer.Build(journal.NewLoader().ResolveBytes("", []byte(content)))
	if len(a.Dates) != 0 {
		t.Fatalf("setup: want 0 dates, got %d", len(a.Dates))
	}
	items := cmplItems(a, cmplAccount, content, len(content), len(content))
	if len(items) == 0 {
		t.Fatal("expected the directive-defined account to complete")
	}
	if items[0].Label != "expenses:food" {
		t.Errorf("label = %q, want expenses:food", items[0].Label)
	}
}

// Golden

func TestCompletionTxtar(t *testing.T) {
	tests := []string{
		"completion-journal",
		"completion-unicode",
		"completion-crlf",
	}

	for _, tt := range tests {
		ar := golden.Read(t, tt)

		t.Run(tt, func(t *testing.T) {
			content := string(ar.Get("in.journal"))

			var cursors []int
			for {
				m := strings.Index(content, "^")
				if m < 0 {
					break
				}
				cursors = append(cursors, m)
				content = content[:m] + content[m+1:]
			}
			if len(cursors) == 0 {
				t.Fatal("no '^' markers in in.journal")
			}

			srv := NewServer("test")
			srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
			srv.server.current = analyzer.Build(srv.server.loader.ResolveBytes("", []byte(content)))

			var b strings.Builder
			for _, c := range cursors {
				line, col := lsputil.LineCol(content, c)
				res, err := srv.server.Completion(t.Context(), &protocol.CompletionParams{
					TextDocumentPositionParams: protocol.TextDocumentPositionParams{
						TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
						Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				ctx, start := detectCompletionCtx(content, c)
				fmt.Fprintf(&b, "%d:%d %s %q\n", line, col, ctx, content[start:c])
				list, ok := res.(*protocol.CompletionList)
				if !ok {
					t.Fatalf("Completion returned %T, want *protocol.CompletionList", res)
				}
				for _, item := range list.Items {
					fmt.Fprintf(&b, "  %s\n", item.Label)
				}
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

// Benchmark

func BenchmarkCompletion(b *testing.B) {
	path := "../../journal/testdata/journals/actual-1ktxns-100accts.journal"
	rj, err := journal.NewLoader().Resolve(path)
	if err != nil {
		b.Fatal(err)
	}
	a := analyzer.Build(rj)
	content := string(rj.Occurrences[0].Src)

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
	srv.server.current = a

	for _, tc := range []struct {
		name string
		pos  int
	}{
		{"1k txns, account", strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2},
		{"1k txns, empty payee", strings.Index(content, "transaction 1") + len("transaction ")},
		{"1k txns, commodity", strings.Index(content, "2 B @@") + len("2 B")},
	} {
		b.Run(tc.name, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tc.pos)
			params := &protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
			}
			// warm up: first request parses the journal lazily
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
