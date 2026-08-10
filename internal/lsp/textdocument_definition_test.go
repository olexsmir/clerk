package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
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
	tests := []string{
		"definition-journal",
		"definition-include",
	}
	for _, tt := range tests {
		ar := golden.Read(t, tt)
		t.Run(tt, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range ar.Files {
				if f.Name != "expect" {
					if err := os.WriteFile(filepath.Join(dir, f.Name), f.Data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}

			content, cursors := stripCursors(string(ar.Get("in.journal")))
			u := uri.File(filepath.Join(dir, "in.journal"))

			srv := NewServer("test")
			srv.server.openDoc(u, content, 1, "journal")

			var b strings.Builder
			for _, c := range cursors {
				pos := lsputil.Position(content, c)
				res, err := srv.server.Definition(t.Context(), &protocol.DefinitionParams{
					TextDocumentPositionParams: protocol.TextDocumentPositionParams{
						TextDocument: protocol.TextDocumentIdentifier{URI: u},
						Position:     pos,
					},
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

func stripCursors(content string) (string, []int) {
	var cursors []int
	for {
		i := strings.Index(content, "^")
		if i < 0 {
			return content, cursors
		}
		cursors = append(cursors, i)
		content = content[:i] + content[i+1:]
	}
}
