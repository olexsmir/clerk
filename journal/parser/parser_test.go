package parser

import (
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
)

func TestParser_ParseFile(t *testing.T) {
	tests := []string{
		"blank line",
		"comment semicolon",
		"comment hash",
		"comment percent",
		"comment star",
		"alias directive",
		"tag directive",
		"year directive",
		"decimal-mark directive",
		"C directive",
		"D directive",
		"P directive",
		"P directive with time",
		"N directive",
		"inclue directive",
		"inclue directive with comment",
		"inclue directive digit path",
		"apply tag directive",
		"apply fixed directive",
		"end apply directive",
		"comment directive",
		"comment directive end alone",
		"comment directive with header",
		"empty comment block",
		"never ending comment directive",
		"nested apply tag directives",
		"account directive",
		"account directive with comment",
		"account with subdirectives",
		"comodity directive",
		"comodity directive word",
		"comodity directive no space",
		"commodity quantity first",
		"commodity quantity after",
		"commodity with subdirectives",
		"payee directive",
		"transaction",
		"automated transaction",
		"transaction with payee",
		"transaction with digit payees",
		"account with spaces",
		"transaction with multiword payee",
		"transaction pending",
		"transaction clearerd",
		"transaction with note",
		"transaction with comment",
		"transaction with secondary date",
		"transaction with costs",
		"transaction with cost and assertion",
		"transaction with posting",
		"transaction with unicode commodity symbols",
		"transaction with quoted commodity",
		"transaction with strange commodity symbols",
		"special chars in description",
		"transaction with tabs",
		"transaction in ukrainian",
		"transaction with code",
		"transaction with posting amounts",
		"transaction with spaced account name",
		"transaction with inline comment",
		"transaction with header comment",
		"transaction with trailing indent",
		"transaction with balance assertion",
		"transaction with inclusive balance assertion",
		"transaction with balance assignment",
		"transaction with scientific amounts",
		"transaction with virtual accounts",
		"virtual postings with statuses",
		"period transaction expressions",
		"unexpected token",
		"recovery after bad posting",
		"illegal only",
		"illegal at start",
		"illegal in posting",
		"illegal between transactions",
		"multiple bad postings",
		"three bad postings",
		"quoted payee names",
		"digit group marks",
		"directive prefixes",
		"bad between good",
		"all postings bad",
		"bad then next transaction",
		"comment between bad postings",
		"bad posting at end",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			a := golden.Read(t, tt)
			l := lexer.New("j", a.Get("input"))
			f := New(l).ParseJournal()
			golden.Assert(t, a, ast.Dump(f))
		})
	}
}

func FuzzParser(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("account expenses:food\n"))
	f.Add([]byte("account a\n  ; subdirective\n"))
	f.Add([]byte("commodity 1,000.00 UAH\n"))
	f.Add([]byte("include other.journal\n"))
	f.Add([]byte("alias checking = assets:bank:checking\n"))
	f.Add([]byte("2024/01/01 * groceries\n    expenses:food  $10.00\n    assets:checking\n"))
	f.Add([]byte("2024/01/01=2024/01/02 groceries\n"))
	f.Add([]byte("2024/01/01 groceries\n  expenses:food  $10.00\n  assets:checking\n"))
	f.Add([]byte("2008/06/03 * eat & shop\n    expenses:food      $1\n    expenses:supplies  $1\n    assets:cash\n"))
	f.Add([]byte("2015-01-03 * Money exchange office\n    Assets:Cash  -20 EUR @ 7.53 HRK\n    Assets:Cash  150.60 HRK\n"))
	f.Add([]byte("2024/01/01 t ; inline comment\n  a  $10\n"))
	f.Add([]byte("2024/01/01 t\n  (a)  10 @@ $20\n  [b]  30\n"))
	f.Add([]byte("2024/01/01 ß\n  (ß)  10 ß\n"))
	f.Add([]byte("2024/01/01 t\n  (! a)  10\n"))
	f.Add([]byte("2024/01/01 t\n  a  $10 == $10\n"))
	f.Add([]byte("  2024/01/01 t\n  a  $10\n"))
	f.Add([]byte("P 2024/01/01 USD 41.50 UAH\n"))
	f.Add([]byte("P 2024-01-01 12:00:00 USD 41.50 UAH\n"))
	f.Add([]byte("P 2024-01-01 12:00 USD 41.50 UAH\n"))
	f.Add([]byte("~ monthly\n    expenses:food  $100\n    assets:checking\n"))
	f.Add([]byte("= /^Income/\n  expenses:food  $10\n"))
	f.Add([]byte("; a comment\n"))
	f.Add([]byte("comment\nbody\nend\n"))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte("перевірка\n"))
	f.Add([]byte("N $\n"))
	f.Add([]byte("apply tag foo\nend\n"))
	f.Add([]byte("@@@\n"))
	f.Add([]byte("   \n"))
	f.Add([]byte("0\n"))
	f.Add([]byte{0xff, 0xfe, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		l := lexer.New("j", data)
		j := New(l).ParseJournal()
		if j == nil {
			t.Fatalf("nil journal for input %q", string(data))
		}

		// error spans must be in bounds (allow sentinel extension past input)
		dataLen := len(data)
		for _, e := range j.Errors {
			if e.Span.Start.Offset < 0 {
				t.Fatal("error span start is negative")
			}
			if e.Span.End.Offset > dataLen+1 {
				t.Fatalf("error span end out of bounds: [%d,%d] len=%d", e.Span.Start.Offset, e.Span.End.Offset, dataLen)
			}
			if len(e.Message) == 0 {
				t.Fatal("empty error message")
			}
		}

		// dump must not panic and must be deterministic
		dump1, dump2 := ast.Dump(j), ast.Dump(j)
		if dump1 != dump2 {
			t.Fatal("non-deterministic dump")
		}
	})
}
