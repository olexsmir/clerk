package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestDefinition_DocumentNotFound(t *testing.T) {
	srv := NewServer("test")
	res, err := srv.server.Definition(t.Context(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///nonexistent.journal")},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("got %v, want nil", res)
	}
}

func TestGolden_Definition(t *testing.T) {
	for _, tt := range []string{"definition-journal", "definition-include"} {
		ar := golden.Read(t, tt)
		t.Run(tt, func(t *testing.T) {
			h := newTxtarHarness(t, ar)

			var b strings.Builder
			for i := range h.cursors {
				pos := h.textDocumentPosition(i).Position
				res, err := h.srv.Definition(t.Context(), &protocol.DefinitionParams{
					TextDocumentPositionParams: h.textDocumentPosition(i),
				})
				if err != nil {
					t.Fatal(err)
				}
				locs, _ := res.(protocol.LocationSlice)
				if len(locs) == 0 {
					fmt.Fprintf(&b, "%d:%d <none>\n", pos.Line, pos.Character)
					continue
				}
				r := locs[0].Range
				fmt.Fprintf(&b, "%d:%d %s %d:%d-%d:%d\n", pos.Line, pos.Character,
					filepath.Base(locs[0].URI.Path()),
					r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

func BenchmarkDefinition(b *testing.B) {
	path := "../../journal/testdata/journals/actual-1ktxns-100accts.journal"
	abs, err := filepath.Abs(path)
	if err != nil {
		b.Fatal(err)
	}
	rj, err := journal.NewLoader().Resolve(abs)
	if err != nil {
		b.Fatal(err)
	}
	a := analyzer.Build(rj)
	content := string(rj.Occurrences[0].Src)

	srv := NewServer("test")
	u := uri.File(abs)
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.current = a

	for tname, tt := range map[string]int{
		"1k txns, account":   strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2,
		"1k txns, payee":     strings.Index(content, "transaction 1") + len("transaction "),
		"1k txns, commodity": strings.Index(content, "2 B @@") + len("2 B"),
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt)
			params := &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: u},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
			}
			// warm up: first request resolves the symbol lazily; assert it found one
			res, err := srv.server.Definition(b.Context(), params)
			if err != nil {
				b.Fatal(err)
			}
			locs, ok := res.(protocol.LocationSlice)
			if !ok || len(locs) == 0 {
				b.Fatalf("%s: no definition", tname)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.Definition(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
