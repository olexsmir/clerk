package lsp

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
)

func TestEncodeSemTokens(t *testing.T) {
	tests := map[string]struct {
		tokens []semanticToken
		want   []uint32
	}{
		"nil":   {nil, nil},
		"empty": {[]semanticToken{}, nil},
		"single": {[]semanticToken{
			{line: 0, col: 0, length: 4, tokenType: semDate},
		}, []uint32{0, 0, 4, semDate, 0}},
		"line 0 col 0": {[]semanticToken{
			{line: 0, col: 0, length: 1, tokenType: semDirective},
		}, []uint32{0, 0, 1, semDirective, 0}},
		"multiple": {[]semanticToken{
			{line: 0, col: 0, length: 10, tokenType: semDate},
			{line: 0, col: 11, length: 5, tokenType: semString},
			{line: 1, col: 4, length: 10, tokenType: semAccount},
		}, []uint32{
			0, 0, 10, semDate, 0,
			0, 11, 5, semString, 0,
			1, 4, 10, semAccount, 0,
		}},
		// input must be sorted by line and column (rawToSemanticTokens output);
		// deltas would underflow otherwise
		"sorted": {[]semanticToken{
			{line: 0, col: 0, length: 10, tokenType: semDate},
			{line: 0, col: 32, length: 2, tokenType: semAmount},
			{line: 0, col: 36, length: 3, tokenType: semCommodity},
			{line: 1, col: 4, length: 6, tokenType: semAccount},
		}, []uint32{
			0, 0, 10, semDate, 0,
			0, 32, 2, semAmount, 0,
			0, 4, 3, semCommodity, 0,
			1, 4, 6, semAccount, 0,
		}},
	}

	for tname, tt := range tests {
		t.Run(tname, func(t *testing.T) {
			if got := encodeSemTokens(tt.tokens); !slices.Equal(got, tt.want) {
				t.Errorf("encodeSemTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemanticTokensEdits(t *testing.T) {
	tests := map[string]struct{ old, new []uint32 }{
		"identical":      {[]uint32{1, 2, 3}, []uint32{1, 2, 3}},
		"both empty":     {},
		"empty to non":   {nil, []uint32{1, 2}},
		"non to empty":   {[]uint32{1, 2}, nil},
		"pure insert":    {[]uint32{1, 2}, []uint32{1, 2, 3, 4}},
		"pure delete":    {[]uint32{1, 2, 3, 4}, []uint32{1, 2}},
		"replace middle": {[]uint32{1, 2, 3, 4, 5}, []uint32{1, 2, 9, 4, 5}},
		"replace all":    {[]uint32{1, 2}, []uint32{3, 4}},
		"replace tail":   {[]uint32{1, 2, 3, 4, 5}, []uint32{1, 2, 3, 4, 6}},
	}
	for tname, tt := range tests {
		t.Run(tname, func(t *testing.T) {
			edits := semanticTokensEdits(tt.old, tt.new)
			if got := applySemEdits(tt.old, edits); !slices.Equal(got, tt.new) {
				t.Errorf("apply(%v, %v) = %v, want %v", tt.old, edits, got, tt.new)
			}
			for _, e := range edits {
				if e.Start+e.DeleteCount > uint32(len(tt.old)) {
					t.Errorf("edit %+v out of bounds for old length %d", e, len(tt.old))
				}
			}
		})
	}
}

func TestServer_Semantic_EmptyDocument(t *testing.T) {
	srv := newServer(t)
	srv.server.openDoc(uri.URI("file:///empty.journal"), "", 1, "journal")
	result, err := srv.server.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///empty.journal")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected empty data for empty doc, got %d values", len(result.Data))
	}
}

func TestServer_Semantic_DocumentNotFound(t *testing.T) {
	result, err := newServer(t).server.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///unknown.journal")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Data != nil {
		t.Errorf("expected nil Data for unknown doc, got %v", result.Data)
	}
}

func TestGolden_SemanticTokens(t *testing.T) {
	for _, tt := range []string{"semantic-empty", "semantic-journal", "semantic-directives", "semantic-unparseable", "semantic-with-errors"} {
		ar := golden.Read(t, tt)

		t.Run(tt+"_golden", func(t *testing.T) {
			toks := renderSemanticTokens(tokSem(ar.Get("in.journal")))
			golden.Assert(t, ar, toks)
		})

		t.Run(tt+"_no-overlap", func(t *testing.T) {
			assertGoldenNoOverlap(t, tt, ar)
		})
	}
}

func TestGolden_SemanticTokensRange(t *testing.T) {
	ar := golden.Read(t, "semantic-range")
	in := ar.Get("in.journal")

	t.Run("no-overlap", func(t *testing.T) {
		assertGoldenNoOverlap(t, "semantic-range", ar)
	})

	t.Run("golden", func(t *testing.T) {
		u := uri.URI("file:///test.journal")
		srv := newServer(t)
		srv.server.openDoc(u, string(in), 1, "journal")

		var out strings.Builder
		for line := range strings.SplitSeq(string(ar.Get("ranges.txt")), "\n") {
			if line == "" {
				continue
			}
			var name string
			var start, end uint32
			if _, err := fmt.Sscanf(line, "%s %d %d", &name, &start, &end); err != nil {
				t.Fatalf("ranges.txt: %q: %v", line, err)
			}

			res, err := srv.server.SemanticTokensRange(t.Context(), &protocol.SemanticTokensRangeParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Range: protocol.Range{
					Start: protocol.Position{Line: start},
					End:   protocol.Position{Line: end},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			tokens := decodeSemTokens(res.Data)
			for _, tok := range tokens {
				if tok.line < start || tok.line > end {
					t.Fatalf("%s: token %d:%d outside range lines [%d,%d]", name, tok.line, tok.col, start, end)
				}
			}
			fmt.Fprintf(&out, "== %s ==\n", name)
			out.WriteString(renderSemanticTokens(tokens))
		}
		golden.Assert(t, ar, out.String())
	})
}

func TestGolden_SemanticTokensDelta(t *testing.T) {
	for _, tt := range []string{"semantic-delta-edit", "semantic-delta-nochange", "semantic-delta-stale"} {
		ar := golden.Read(t, tt)

		t.Run(tt+"_no-overlap", func(t *testing.T) {
			assertGoldenNoOverlap(t, tt, ar)
		})

		t.Run(tt, func(t *testing.T) {
			in := ar.Get("in.journal")

			u := uri.URI("file:///test.journal")
			srv := newServer(t)
			srv.server.openDoc(u, string(in), 1, "journal")

			full, err := srv.server.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
			})
			if err != nil {
				t.Fatal(err)
			}
			if full.ResultID == nil || *full.ResultID == "" {
				t.Fatal("expected a resultId on the full result")
			}

			finalText := in
			if ed := ar.Get("edited.journal"); ed != nil {
				srv.server.updateDoc(u, 2, partialChangeFromDiff(string(in), string(ed)))
				finalText = ed
			}
			prev := *full.ResultID
			if p := ar.Get("prev-id"); p != nil {
				prev = strings.TrimSpace(string(p))
			}

			res, err := srv.server.SemanticTokensFullDelta(t.Context(), &protocol.SemanticTokensDeltaParams{
				TextDocument:     protocol.TextDocumentIdentifier{URI: u},
				PreviousResultID: prev,
			})
			if err != nil {
				t.Fatal(err)
			}

			var client []uint32
			var out strings.Builder
			switch r := res.(type) {
			case *protocol.SemanticTokensDelta:
				fmt.Fprintf(&out, "delta resultId %s\n", *r.ResultID)
				client = applySemEdits(full.Data, r.Edits)
			case *protocol.SemanticTokens:
				fmt.Fprintf(&out, "full resultId %s\n", *r.ResultID)
				client = r.Data
			default:
				t.Fatalf("unexpected result type %T", res)
			}

			// The client's token state must equal the final text's tokens: an
			// independent reference that keeps the golden from capturing bugs.
			if want := encodeSemTokens(tokSem(finalText)); !slices.Equal(client, want) {
				t.Errorf("delta flow produced %d elems, want %d", len(client), len(want))
			}

			out.WriteString(renderSemanticTokens(tokSem(finalText)))
			golden.Assert(t, ar, out.String())
		})
	}
}

func assertGoldenNoOverlap(t *testing.T, tt string, ar *golden.Archive) {
	t.Helper()
	for _, f := range ar.Files {
		if strings.HasSuffix(f.Name, ".journal") {
			assertNoOverlap(t, tt, tokSem(f.Data))
		}
	}
}

func assertNoOverlap(t *testing.T, tt string, toks []semanticToken) {
	t.Helper()
	slices.SortFunc(toks, func(a, b semanticToken) int {
		if a.line != b.line {
			return int(a.line) - int(b.line)
		}
		return int(a.col) - int(b.col)
	})
	for i := 1; i < len(toks); i++ {
		prev, cur := toks[i-1], toks[i]
		if prev.line != cur.line {
			continue
		}
		if cur.col < prev.col+prev.length {
			t.Errorf("%s: overlapping tokens on line %d: %s@%d+%d then %s@%d+%d",
				tt, prev.line, tokenTypeStrings[prev.tokenType], prev.col, prev.length,
				tokenTypeStrings[cur.tokenType], cur.col, cur.length)
		}
	}
}

func renderSemanticTokens(tokens []semanticToken) string {
	slices.SortFunc(tokens, func(a, b semanticToken) int {
		if a.line != b.line {
			return int(a.line) - int(b.line)
		}
		return int(a.col) - int(b.col)
	})
	var b strings.Builder
	for _, tok := range tokens {
		fmt.Fprintf(&b, "%d:%d+%d %s", tok.line, tok.col, tok.length, tokenTypeStrings[tok.tokenType])
		for i, m := range modifierStrings {
			if tok.modifiers&(1<<uint(i)) != 0 {
				b.WriteByte(' ')
				b.WriteString(m)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func decodeSemTokens(data []uint32) []semanticToken {
	var out []semanticToken
	line, col := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		if data[i] > 0 {
			line += int(data[i])
			col = int(data[i+1])
		} else {
			col += int(data[i+1])
		}
		out = append(out, semanticToken{
			line:      uint32(line),
			col:       uint32(col),
			length:    data[i+2],
			tokenType: data[i+3],
			modifiers: data[i+4],
		})
	}
	return out
}

func tokSem(content []byte) []semanticToken {
	c := string(content)
	return tokenizeForSemantics(c, parseJournalStr(c))
}

func TestSemanticTokensIncremental(t *testing.T) {
	for _, tt := range []string{"semantic-incremental-append", "semantic-incremental-insert", "semantic-incremental-delete", "semantic-incremental-inline", "semantic-incremental-header"} {
		t.Run(tt, func(t *testing.T) {
			ar := golden.Read(t, tt)
			in, edited := ar.Get("in.journal"), ar.Get("edited.journal")

			// golden: the server's incremental result equals a full tokenization
			golden.Assert(t, ar, renderSemanticTokens(serverSemTokensAfterEdit(t, newServer(t).server, string(in), string(edited))))

			// engage: the incremental path actually runs, not a silent full rebuild
			ev := partialChangeFromDiff(string(in), string(edited))[0].(*protocol.TextDocumentContentChangePartial)
			start, oldEnd, newEnd, delta := editRegion(string(in), ev)
			_, entries := computeSemTokens(string(in), parseJournalStr(string(in)))
			if _, ok := incrementalTokens(string(edited), parseJournalStr(string(edited)), entries, semEdit{start: start, oldEnd: oldEnd, newEnd: newEnd, deltaLine: delta}); !ok {
				t.Error("incremental path not taken")
			}
		})
	}

	t.Run("recover", func(t *testing.T) {
		ar := golden.Read(t, "semantic-incremental-recover")
		bad, good := ar.Get("in.journal"), ar.Get("edited.journal")
		golden.Assert(t, ar, renderSemanticTokens(serverSemTokensAfterEdit(t, newServer(t).server, string(bad), string(good))))
	})
}

func BenchmarkSemanticTokens(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")

	// Cold path: each iteration re-parses and re-encodes, as after an edit.
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tokens := tokenizeForSemantics(content, parseJournalStr(content))
		_ = encodeSemTokens(tokens)
	}
}

// BenchmarkSemanticTokensDelta measures the cost of one dela response after an edit.
func BenchmarkSemanticTokensDelta(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	prev := encodeSemTokens(tokenizeForSemantics(content, parseJournalStr(content)))
	edited := content + "\n2000-06-15 transaction 2501\n  expenses:new  1 C\n  assets:cash\n"

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data := encodeSemTokens(tokenizeForSemantics(edited, parseJournalStr(edited)))
		_ = semanticTokensEdits(prev, data)
	}
}

func BenchmarkSemanticTokensEdits(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	old := encodeSemTokens(tokenizeForSemantics(content, parseJournalStr(content)))
	new_ := encodeSemTokens(tokenizeForSemantics(
		content+"\n2000-06-15 transaction 2501\n  expenses:new  1 C\n  assets:cash\n",
		parseJournalStr(content+"\n2000-06-15 transaction 2501\n  expenses:new  1 C\n  assets:cash\n"),
	))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = semanticTokensEdits(old, new_)
	}
}

func BenchmarkIncrementalTokens(b *testing.B) {
	content := openJournal(b, "../../journal/testdata/journals/actual-1ktxns-100accts.journal")
	_, entries := computeSemTokens(content, parseJournalStr(content))

	newTx := "\n2024-01-01 new  $1\n    expenses:new  $1\n    assets:cash"
	appendEdit := content + newTx + "\n"
	first := strings.Index(content, "\n\n")
	mid := strings.Index(content[first+2:], "\n\n") + first + 2 // blank line between the first two transactions
	insertEdit := content[:mid] + newTx + content[mid:]

	b.ReportAllocs()
	for _, tc := range []struct{ name, edited string }{
		{"append", appendEdit},
		{"insert", insertEdit},
	} {
		ev := partialChangeFromDiff(content, tc.edited)[0].(*protocol.TextDocumentContentChangePartial)
		start, oldEnd, newEnd, delta := editRegion(content, ev)
		j := parseJournalStr(tc.edited)

		b.Run(tc.name+"/full", func(b *testing.B) {
			for b.Loop() {
				_ = tokenizeForSemantics(tc.edited, j)
			}
		})
		b.Run(tc.name+"/incremental", func(b *testing.B) {
			for b.Loop() {
				if _, ok := incrementalTokens(tc.edited, j, entries, semEdit{start: start, oldEnd: oldEnd, newEnd: newEnd, deltaLine: delta}); !ok {
					b.Fatal("fell back to full")
				}
			}
		})
	}
}

func applySemEdits(data []uint32, edits []protocol.SemanticTokensEdit) []uint32 {
	out := slices.Clone(data)
	for _, e := range edits {
		out = append(append(out[:e.Start], e.Data...), out[e.Start+e.DeleteCount:]...)
	}
	return out
}

func partialChangeFromDiff(from, to string) []protocol.TextDocumentContentChangeEvent {
	p := 0
	for p < len(from) && p < len(to) && from[p] == to[p] {
		p++
	}
	s := 0
	for s < len(from)-p && s < len(to)-p && from[len(from)-1-s] == to[len(to)-1-s] {
		s++
	}
	return []protocol.TextDocumentContentChangeEvent{
		&protocol.TextDocumentContentChangePartial{
			Range: protocol.Range{
				Start: lsputil.Position(from, p),
				End:   lsputil.Position(from, len(from)-s),
			},
			Text: to[p : len(to)-s],
		},
	}
}

func editRegion(from string, ev *protocol.TextDocumentContentChangePartial) (start, oldEnd, newEnd, deltaLine int) {
	li := lsputil.NewLineIndex(from)
	start = li.Offset(int(ev.Range.Start.Line), int(ev.Range.Start.Character))
	oldEnd = li.Offset(int(ev.Range.End.Line), int(ev.Range.End.Character))
	newEnd = start + len(ev.Text)
	deltaLine = strings.Count(ev.Text, "\n") - strings.Count(from[start:oldEnd], "\n")
	return
}

func serverSemTokensAfterEdit(t *testing.T, srv *server, in, edited string) []semanticToken {
	t.Helper()
	u := uri.URI("file:///test.journal")
	srv.openDoc(u, in, 1, "journal")
	if _, ok := srv.tokensForDoc(u); !ok {
		t.Fatal("priming failed")
	}
	srv.updateDoc(u, 2, partialChangeFromDiff(in, edited))
	toks, ok := srv.tokensForDoc(u)
	if !ok {
		t.Fatal("tokensForDoc failed after edit")
	}
	return toks
}

func openJournal(t testing.TB, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}
