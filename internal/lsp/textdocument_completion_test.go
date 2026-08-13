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

func TestGolden_Completion(t *testing.T) {
	for _, tt := range []string{"completion-contexts", "completion-journal", "completion-no-transactions"} {
		ar := golden.Read(t, tt)

		t.Run(tt, func(t *testing.T) {
			h := newTxtarHarness(t, ar)

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
				for _, item := range list.Items {
					fmt.Fprintf(&b, "  %s\n", item.Label)
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

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")
	srv.server.analysisFor(uri.URI("file:///test.journal")) // warm the per-doc cache

	for tname, tt := range map[string]int{
		"1k txns, account":     strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2,
		"1k txns, empty payee": strings.Index(content, "transaction 1") + len("transaction "),
		"1k txns, commodity":   strings.Index(content, "2 B @@") + len("2 B"),
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt)
			params := &protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
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
