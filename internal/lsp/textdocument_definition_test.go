package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/testutil/golden"
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

// Golden

func TestDefinitionTxtar(t *testing.T) {
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
