package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestValidateRenameName(t *testing.T) {
	valid := map[string][]string{
		"account":   {"expenses", "expenses:food", "a:b:c"},
		"commodity": {"$", "€", "USD", "US$"},
		"payee":     {"Grocery Store", "acme", "продукти"},
		"tag":       {"client", "a-b_c", "x123"},
	}

	invalid := map[string][]string{
		"account":   {"", " expenses", "expenses ", "expenses;x", "expenses\nx", "expenses\r"},
		"commodity": {"", " $", "$ ", "$;x", "$\n"},
		"payee":     {"", " payee", "payee|note", "payee;x"},
		"tag":       {"", "a:b", "a,b", "a b", "a;b", "tag\tx"},
	}

	validators := map[string]func(string) error{
		"account":   validateAccountName,
		"commodity": validateCommodityName,
		"payee":     validatePayeeName,
		"tag":       validateTagName,
	}

	for kind, v := range validators {
		for _, name := range valid[kind] {
			if err := v(name); err != nil {
				t.Errorf("%s %q: unexpected error %v", kind, name, err)
			}
		}
		for _, name := range invalid[kind] {
			if err := v(name); err == nil {
				t.Errorf("%s %q: expected error", kind, name)
			}
		}
	}
}

func TestAccountMatches(t *testing.T) {
	tests := map[string]bool{
		"expenses":        true,
		"expenses:food":   true,
		"expenses:food:l": true,
		"expenses2":       false,
		"ex":              false,
		"other":           false,
	}
	for name, want := range tests {
		if got := accountMatches(name, "expenses"); got != want {
			t.Errorf("accountMatches(%q, expenses) = %v, want %v", name, got, want)
		}
	}
}

// Golden

func TestGolden_Rename(t *testing.T) {
	for _, tt := range []string{"rename-account", "rename-commodity", "rename-payee", "rename-tag"} {
		ar := golden.Read(t, tt)
		t.Run(tt, func(t *testing.T) {
			h := newTxtarHarness(t, ar)

			newNames := strings.Split(strings.TrimRight(string(ar.Get("rename")), "\n"), "\n")
			if len(newNames) != len(h.cursors) {
				t.Fatalf("%d cursors, %d rename names", len(h.cursors), len(newNames))
			}

			var b strings.Builder
			for i := range h.cursors {
				tdp := h.textDocumentPosition(i)
				prep, err := h.srv.PrepareRename(t.Context(), &protocol.PrepareRenameParams{TextDocumentPositionParams: tdp})
				if err != nil {
					t.Fatal(err)
				}
				edit, err := h.srv.Rename(t.Context(), &protocol.RenameParams{
					TextDocumentPositionParams: tdp,
					NewName:                    newNames[i],
				})
				if err != nil {
					t.Fatal(err)
				}

				fmt.Fprintf(&b, "%d:%d\n", tdp.Position.Line, tdp.Position.Character)
				if p, ok := prep.(*protocol.PrepareRenamePlaceholder); ok {
					r := p.Range
					fmt.Fprintf(&b, "  prep in.journal %d:%d-%d:%d %q\n",
						r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, p.Placeholder)
				} else {
					b.WriteString("  prep <none>\n")
				}
				if edit == nil {
					b.WriteString("  ren <none>\n")
					continue
				}
				uris := make([]string, 0, len(edit.Changes))
				for u := range edit.Changes {
					uris = append(uris, string(u))
				}
				sort.Strings(uris)
				for _, us := range uris {
					for _, e := range edit.Changes[uri.URI(us)] {
						r := e.Range
						fmt.Fprintf(&b, "  ren %s %d:%d-%d:%d %q\n", filepath.Base(uri.URI(us).Path()),
							r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, e.NewText)
					}
				}
			}
			golden.Assert(t, ar, b.String())
		})
	}
}

func BenchmarkRename(b *testing.B) {
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

	for tname, tt := range map[string]struct {
		newName string
		pos     int
	}{
		"1k txns, account":   {"1:2:3x", strings.Index(content, "\n  1:2:3 ") + len("\n  ") + 2},
		"1k txns, payee":     {"transaction 1x", strings.Index(content, "transaction 1") + len("transaction ")},
		"1k txns, commodity": {"Bx", strings.Index(content, "2 B @@") + len("2 B")},
	} {
		b.Run(tname, func(b *testing.B) {
			line, col := lsputil.LineCol(content, tt.pos)
			params := &protocol.RenameParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: u},
					Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
				NewName: tt.newName,
			}
			// warm up: first request generates the workspace edits; assert it found one
			edit, err := srv.server.Rename(b.Context(), params)
			if err != nil {
				b.Fatal(err)
			}
			if edit == nil || len(edit.Changes) == 0 {
				b.Fatalf("%s: no edits", tname)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.Rename(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// txtarHarness runs per-cursor lsp requests againt golden tests
type txtarHarness struct {
	srv     *server
	uri     uri.URI
	content string
	cursors []int
}

func newTxtarHarness(t *testing.T, ar *golden.Archive) *txtarHarness {
	t.Helper()
	dir := t.TempDir()
	for _, f := range ar.Files {
		if f.Name == "expect" || f.Name == "rename" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	content, cursors := stripCursors(string(ar.Get("in.journal")))
	if len(cursors) == 0 {
		t.Fatal("no '^' markers in in.journal")
	}
	u := uri.File(filepath.Join(dir, "in.journal"))

	srv := NewServer("test")
	srv.server.openDoc(u, content, 1, "journal")

	return &txtarHarness{srv: srv.server, uri: u, content: content, cursors: cursors}
}

func (h *txtarHarness) textDocumentPosition(i int) protocol.TextDocumentPositionParams {
	return protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: h.uri},
		Position:     lsputil.Position(h.content, h.cursors[i]),
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
