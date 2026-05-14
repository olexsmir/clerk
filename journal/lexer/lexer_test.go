package lexer

import (
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal/token"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple transaction", `2024/01/01 groceries
    expenses:food  $10.00
    assets:checking
`},
		{"transaction, accounts with uppercase latters", `
2011/01/27 Book Store
    Expenses:Books                       $20.00
    Liabilities:MasterCard
`},
		{"cleared transaction", `2024/01/01 * groceries
    expenses:food  $10.00
    assets:checking
`},
		{"automated transaction", `= ^income
    (liabilities:tax)  *.33

= expenses:gifts
    budget:gifts  (amount * -1)
`},
		{"transaction with code", `2024/01/01 (123) groceries
    expenses:food  $10.00
    assets:checking
`},
		{"transaction with virtual accounts", `2024/01/01 * groceries
	(virtual:account)  1 PESO
	[something:else]   5 PESO
`},
		{"transaction with unicode commodity symbols", `2024/01/01 groceries
    expenses:food  €10.00
    expenses:food  £5.00
    expenses:food  ₹700.00
    expenses:food  40.00 гривні
    assets:cash
`},
		{"date with secondary", `2024/01/01=2024/01/02 groceries`},
		{"better date", `2024-01-02`},
		{"comment line", `; this is a comment`},
		{"star comment", `* this is a comment`},
		{"hash comment", `# this is a comment`},
		{"account directive", `account expenses:food`},
		{"commodity directive", `commodity 1,000.00 UAH`},
		{"market price directive", "P 2024-01-01 USD 40.50 UAH\n"},
		{"market price directive with time", "P 2024-01-01 12:00:00 USD 40.50 UAH\n"},
		{"inline comment", `2024/01/01 groceries ; a note`},
		{"empty", ``},
		{"blank lines", "\n\n\n"},
		{"comment block directive", "comment\ncontent\nend\n"},
		{"comment block directive without end", "comment\ncontent\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New("j", []byte(tt.input))
			golden.Assert(t, l.Dump())
		})
	}
}

// token category bounds, ensures fuzzer never sees out-of-range token types.
const maxKnownTokenType = token.N

func FuzzLexer(f *testing.F) {
	f.Add([]byte("2024/01/01 groceries\n  expenses:food  $10.00\n  assets:checking\n"))
	f.Add([]byte("2024/01/01 * groceries\n  expenses:food  $10.00\n  assets:checking\n"))
	f.Add([]byte("2024/01/01 ! groceries\n  expenses:food  $10.00\n  assets:checking\n"))
	f.Add([]byte("2024/01/01 t ; inline comment\n  a  $10\n"))
	f.Add([]byte("2024/01/01 t\n  (a)  10 @@ $20\n  [b]  30\n"))
	f.Add([]byte("2008/06/03 * eat & shop\n    expenses:food      $1\n    expenses:supplies  $1\n    assets:cash\n"))
	f.Add([]byte("2015-01-03 * Money exchange office\n    Assets:Cash  -20 EUR @ 7.53 HRK\n    Assets:Cash  150.60 HRK\n"))
	f.Add([]byte("2024/01/01 ß\n  (ß)  10 ß\n"))
	f.Add([]byte("2024/01/01 t\n  (! a)  10\n"))
	f.Add([]byte("comment\nbody\nend\n"))
	f.Add([]byte("apply tag foo\nend\n"))
	f.Add([]byte("; a comment\n"))
	f.Add([]byte("# a comment\n"))
	f.Add([]byte("* a comment\n"))
	f.Add([]byte("account expenses:food\n"))
	f.Add([]byte("commodity 1,000.00 UAH\n"))
	f.Add([]byte("N $\n"))
	f.Add([]byte("P 2024-01-01 USD 41.50 UAH\n"))
	f.Add([]byte("P 2024-01-01 12:00:00 USD 41.50 UAH\n"))
	f.Add([]byte("P 2024-01-01 12:00 USD 41.50 UAH\n"))
	f.Add([]byte("~ monthly\n  a  $10\n  b\n"))
	f.Add([]byte("= /^Income/\n  expenses:food  $10\n"))
	f.Add([]byte("перевірка\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte("@@@\n"))
	f.Add([]byte("   \n"))
	f.Add([]byte("0\n"))
	f.Add([]byte{0xff, 0xfe, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Pass 1: lex and validate token stream
		l := New("j", data)
		var tokens []token.Token
		maxTokens := max(len(data)*2, 16)
		prevEnd := -1
		for range maxTokens {
			tok := l.Next()

			// Monotonic span
			if tok.Span.Start.Offset < prevEnd {
				t.Fatalf("non-monotonic span: prevEnd=%d current=%s %d",
					prevEnd, tok.Type, tok.Span.Start.Offset)
			}

			// Token type in range (no garbage from memory corruption)
			if tok.Type < 0 || tok.Type > maxKnownTokenType {
				t.Fatalf("token type out of range: %d", tok.Type)
			}

			// Span in bounds (EOF/NEWLINE sentinels may extend one past input)
			maxEnd := len(data)
			if tok.Type == token.NEWLINE || tok.Type == token.EOF {
				maxEnd = len(data) + 1
			}
			if tok.Span.Start.Offset < 0 || tok.Span.End.Offset > maxEnd ||
				tok.Span.Start.Offset > tok.Span.End.Offset {
				t.Fatalf("span out of bounds: [%d,%d] for len=%d type=%s",
					tok.Span.Start.Offset, tok.Span.End.Offset, len(data), tok.Type)
			}

			if tok.Type == token.EOF {
				break
			}

			// Non-zero-length for non-EOF tokens (NEWLINE sentinel is exempt)
			if tok.Type != token.NEWLINE && tok.Span.End.Offset <= tok.Span.Start.Offset {
				t.Fatalf("non-progressing token: %s %q at %d:%d-%d:%d",
					tok.Type, tok.Literal,
					tok.Span.Start.Line, tok.Span.Start.Col,
					tok.Span.End.Line, tok.Span.End.Col)
			}

			tokens = append(tokens, tok)
			prevEnd = tok.Span.End.Offset
		}

		if prevEnd > len(data)+1 {
			t.Fatalf("token consumed beyond input: end=%d len=%d", prevEnd, len(data))
		}

		// Pass 2: re-lex the same input — token stream must be identical
		l2 := New("j", data)
		for _, expected := range tokens {
			tok := l2.Next()
			if tok.Type != expected.Type || tok.Literal != expected.Literal {
				t.Fatalf("re-lex mismatch at offset %d: expected (%s %q), got (%s %q)",
					expected.Span.Start.Offset, expected.Type, expected.Literal, tok.Type, tok.Literal)
			}
		}
	})
}
