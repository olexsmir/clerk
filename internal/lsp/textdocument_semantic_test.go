package lsp

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

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

func TestServer_Semantic_SimpleTransaction(t *testing.T) {
	content := `2024-01-15 test
    expenses:food  $50
    assets:cash
`

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")

	result, err := srv.server.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Data) == 0 {
		t.Fatal("expected non-empty token data")
	}
	if len(result.Data)%5 != 0 {
		t.Fatalf("token data length %d is not a multiple of 5", len(result.Data))
	}
	// first token is the transaction date at line 0, col 0: deltas are 0, 0
	if result.Data[0] != 0 || result.Data[1] != 0 {
		t.Errorf("first token deltas = %d,%d, want 0,0", result.Data[0], result.Data[1])
	}
}

func TestServer_Semantic_EmptyDocument(t *testing.T) {
	srv := NewServer("test")
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
	result, err := NewServer("test").server.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
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

func TestServer_Semantic_Range(t *testing.T) {
	content := `2024-01-15 test
    expenses:food  $50

2024-01-16 other
  expenses:drinks  $20
`

	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")

	result, err := srv.server.SemanticTokensRange(t.Context(), &protocol.SemanticTokensRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 1, Character: 50},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || len(result.Data) == 0 {
		t.Fatal("expected non-empty tokens for range")
	}

	// decode and assert every token is inside the requested line range
	line, col := 0, 0
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc := result.Data[i], result.Data[i+1]
		if dl > 0 {
			col = 0
		}
		line += int(dl)
		col += int(dc)
		if line > 1 {
			t.Fatalf("token at line %d outside requested range [0,1]", line)
		}
	}
}

// Golden

func TestGolden_SemanticTokens(t *testing.T) {
	for _, tt := range []string{"semantic-empty", "semantic-journal", "semantic-directives", "semantic-unparseable", "semantic-with-errors"} {
		ar := golden.Read(t, tt)

		t.Run(tt+"_golden", func(t *testing.T) {
			toks := renderSemanticTokens(tokSem(ar.Get("in.journal")))
			golden.Assert(t, ar, toks)
		})

		t.Run(tt+"_no-overlap", func(t *testing.T) {
			toks := tokSem(ar.Get("in.journal"))
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
		})
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

func tokSem(content []byte) []semanticToken {
	c := string(content)
	return tokenizeForSemantics(c, parseJournalStr(c))
}
