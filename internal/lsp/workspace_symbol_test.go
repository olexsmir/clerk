package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/testutil/golden"
)

func TestSortScoredSymbols(t *testing.T) {
	scored := []scoredSymbol{
		{symbolAccount, "zz:zz", 0.5},
		{symbolTag, "aa", 1.0},
		{symbolPayee, "bb", 0.5},
	}
	sortScoredSymbols(scored)

	want := []string{"aa", "bb", "zz:zz"}
	for i, s := range scored {
		if s.name != want[i] {
			t.Errorf("pos %d = %q, want %q", i, s.name, want[i])
		}
	}
}

func TestServer_Symbols_EmptyQuery(t *testing.T) {
	srv := NewServer("test")
	u := uri.URI("file:///test.journal")
	srv.server.openDoc(u, "account expenses:food\n", 1, "journal")

	res, err := srv.server.Symbols(context.Background(), &protocol.WorkspaceSymbolParams{Query: ""})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("got %v, want nil for empty query", res)
	}
}

func TestGolden_Symbols(t *testing.T) {
	ar := golden.Read(t, "workspace-symbol")
	t.Run("workspace-symbol", func(t *testing.T) {
		h := newTxtarHarness(t, ar)

		// One query per line; a trailing blank line is the empty-query case.
		queries := strings.Split(strings.TrimSuffix(string(ar.Get("queries")), "\n"), "\n")
		var b strings.Builder
		for _, q := range queries {
			res, err := h.srv.Symbols(t.Context(), &protocol.WorkspaceSymbolParams{Query: q})
			if err != nil {
				t.Fatal(err)
			}
			if res == nil {
				fmt.Fprintf(&b, "%q <none>\n", q)
				continue
			}
			list, ok := res.(protocol.WorkspaceSymbolSlice)
			if !ok {
				t.Fatalf("Symbols returned %T, want WorkspaceSymbolSlice", res)
			}
			for _, sym := range list {
				loc, ok := sym.Location.(*protocol.Location)
				if !ok {
					t.Fatalf("Symbol %q location is %T, want *Location", sym.Name, sym.Location)
				}
				r := loc.Range
				fmt.Fprintf(&b, "%q %s %s %s %d:%d-%d:%d\n", q, symbolKindName(sym.Kind), sym.Name,
					filepath.Base(loc.URI.Path()),
					r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
			}
		}
		golden.Assert(t, ar, b.String())
	})
}

func BenchmarkSymbols(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	srv := NewServer("test")
	u := uri.URI("file:///test.journal")
	srv.server.openDoc(u, content, 1, "journal")
	srv.server.analysisFor(u) // warm the per-doc cache

	for tname, tt := range map[string]struct {
		query string
		want  bool // expect non-empty results
	}{
		"account prefix": {"1:2", true},
		"all payees":     {"transaction", true},
		"mixed kinds":    {"B", true},
		"no match":       {"xyz", false},
	} {
		b.Run(tname, func(b *testing.B) {
			params := &protocol.WorkspaceSymbolParams{Query: tt.query}

			// warm up: assert the query matches as expected
			res, err := srv.server.Symbols(b.Context(), params)
			if err != nil {
				b.Fatal(err)
			}
			if (res != nil) != tt.want {
				b.Fatalf("%s: query %q: got res==nil=%v, want %v", tname, tt.query, res == nil, !tt.want)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := srv.server.Symbols(b.Context(), params); err != nil {
					b.Fatal(err)
				}
			}

			// guard: a whole-file re-parse per request (~13ms) would blow past
			// this and must be caught
			if avg := b.Elapsed() / time.Duration(b.N); avg > 5*time.Millisecond {
				b.Fatalf("symbols %v/op: reparse regression", avg)
			}
		})
	}
}

func symbolKindName(k protocol.SymbolKind) string {
	switch k {
	case protocol.SymbolKindClass:
		return "class"
	case protocol.SymbolKindVariable:
		return "variable"
	case protocol.SymbolKindObject:
		return "object"
	case protocol.SymbolKindProperty:
		return "property"
	}
	return "other"
}
