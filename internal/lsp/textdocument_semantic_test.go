package lsp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/testutil/golden"
)

func TestSemanticTokens_Legend(t *testing.T) {
	if len(tokenTypeStrings) != semTypeCount {
		t.Fatalf("tokenTypeStrings has %d entries, want %d (one per TokenType constant)",
			len(tokenTypeStrings), semTypeCount)
	}
	legend := getSemanticTokensLegend()
	if !slices.Equal(legend.TokenTypes, tokenTypeStrings) {
		t.Errorf("legend.TokenTypes = %v, want %v", legend.TokenTypes, tokenTypeStrings)
	}
	if !slices.Equal(legend.TokenModifiers, modifierStrings) {
		t.Errorf("legend.TokenModifiers = %v, want %v", legend.TokenModifiers, modifierStrings)
	}
}

func TestSemanticTokensEncode(t *testing.T) {
	tests := []struct {
		name   string
		tokens []semanticToken
		want   []uint32
	}{
		{"nil", nil, nil},
		{"empty", []semanticToken{}, nil},
		{"single", []semanticToken{
			{line: 0, col: 0, length: 4, tokenType: SemanticDate},
		}, []uint32{0, 0, 4, SemanticDate, 0}},
		{"line 0 col 0", []semanticToken{
			{line: 0, col: 0, length: 1, tokenType: SemanticDirective},
		}, []uint32{0, 0, 1, SemanticDirective, 0}},
		{"multiple", []semanticToken{
			{line: 0, col: 0, length: 10, tokenType: SemanticDate},
			{line: 0, col: 11, length: 5, tokenType: SemString},
			{line: 1, col: 4, length: 10, tokenType: SemanticAccount},
		}, []uint32{
			0, 0, 10, SemanticDate, 0,
			0, 11, 5, SemString, 0,
			1, 4, 10, SemanticAccount, 0,
		}},
		// the wire format requires non-negative deltas; unsorted input must
		// be sorted first (commodity at col 36 comes after amount at col 32)
		{"unsorted input", []semanticToken{
			{line: 0, col: 0, length: 10, tokenType: SemanticDate},
			{line: 0, col: 36, length: 3, tokenType: SemanticCommodity},
			{line: 0, col: 32, length: 2, tokenType: SemanticAmount},
			{line: 1, col: 4, length: 6, tokenType: SemanticAccount},
		}, []uint32{
			0, 0, 10, SemanticDate, 0,
			0, 32, 2, SemanticAmount, 0,
			0, 4, 3, SemanticCommodity, 0,
			1, 4, 6, SemanticAccount, 0,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeSemTokens(tt.tokens); !slices.Equal(got, tt.want) {
				t.Errorf("encodeSemTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemanticTokens_Server_SimpleTransaction(t *testing.T) {
	content := "2024-01-15 test\n    expenses:food  $50\n    assets:cash\n"

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

func TestSemanticTokens_Server_EmptyDocument(t *testing.T) {
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

func TestSemanticTokens_Server_DocumentNotFound(t *testing.T) {
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

func TestSemanticTokens_Server_Range(t *testing.T) {
	content := "2024-01-15 test\n    expenses:food  $50\n2024-01-16 other\n    expenses:drinks  $20\n"
	srv := NewServer("test")
	srv.server.openDoc(uri.URI("file:///test.journal"), content, 1, "journal")

	params := &protocol.SemanticTokensRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///test.journal")},
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 1, Character: 50},
		},
	}
	result, err := srv.server.SemanticTokensRange(context.Background(), params)
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

func TestSemanticTokensTxtar(t *testing.T) {
	tests := []string{
		"semantic-empty",
		"semantic-journal",
		"semantic-directives",
		"semantic-unparseable",
	}

	for _, tt := range tests {
		ar := golden.Read(t, tt)

		t.Run(tt+"_golden", func(t *testing.T) {
			toks := renderSemanticTokens(tokSem(string(ar.Get("in.journal"))))
			golden.Assert(t, ar, toks)
		})

		t.Run(tt+"_no-overlap", func(t *testing.T) {
			toks := tokSem(string(ar.Get("in.journal")))
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

func tokSem(content string) []semanticToken {
	return tokenizeForSemantics(content, parseJournalStr(content))
}
