package lexer

import (
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal/token"
)

func TestLexer(t *testing.T) {
	tests := []string{
		"account directive",
		"automated transaction",
		"better date",
		"blank lines",
		"cleared transaction",
		"comment block directive without end",
		"comment block directive",
		"comment line",
		"commodity directive",
		"date with secondary",
		"empty",
		"hash comment",
		"inline comment",
		"market price directive with time",
		"market price directive",
		"simple transaction",
		"special chars in description",
		"star comment",
		"transaction with code",
		"transaction with unicode commodity symbols",
		"transaction with virtual accounts",
		"transaction, accounts with uppercase latters",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			a := golden.Read(t, tt)
			golden.Assert(t, a, New("j", string(a.Get("input"))).dump())
		})
	}
}

func FuzzLexer(f *testing.F) {
	const maxKnownTokenType = token.C // ensures fuzzer never sees out-of-range token types

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
		l := New("j", string(data))
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
		l2 := New("j", string(data))
		for _, expected := range tokens {
			tok := l2.Next()
			if tok.Type != expected.Type || tok.Literal != expected.Literal {
				t.Fatalf("re-lex mismatch at offset %d: expected (%s %q), got (%s %q)",
					expected.Span.Start.Offset, expected.Type, expected.Literal, tok.Type, tok.Literal)
			}
		}
	})
}
