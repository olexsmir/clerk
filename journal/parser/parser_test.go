package parser

import (
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
)

func TestParser_ParseFile(t *testing.T) {
	tests := []struct{ name, inp string }{
		{"blank line", "\n"},
		{"comment semicolon", "; a comment\n"},
		{"comment hash", "# a comment\n"},
		{"comment percent", "% a comment\n"},
		{"comment star", "* a comment\n"},
		{"alias directive", "alias checking = assets:bank:checking\n"},
		{"tag directive", "tag project-xyz\n"},
		{"year directive", "year 1488\n"},
		{"decimal-mark directive", "decimal-mark ,\n"},
		{"C directive", "C 1.00s = 100c\n"},
		{"D directive", `D $1.00
D 10 UAH
`},
		{"P directive", "P 2024/01/01 USD 40.50 UAH\n"},
		{"P directive with time", "P 2024-01-01 12:00:00 USD 40.50 UAH\n"},
		{"N directive", "N $\n"},
		{"inclue directive", "include ./path-to.journal\n"},
		{"inclue directive with comment", "include ./path.journal ; something importnat\n"},
		{"apply tag directive", "apply tag hashtag\n"},
		{"apply fixed directive", "apply fixed CAD $0.90\n"},
		{"end apply directive", "end apply tag\n"},
		{"comment directive", "comment\nsome text\nend comment\n"},
		{"comment directive end alone", "comment\nsome text\nend\n"},
		{"comment directive with header", "comment tag:hidden\nstuff\nend\n"},
		{"empty comment block", "comment\nend\n"},
		{"never ending comment directive", "comment\nsome text\n"},
		{"nested apply tag directives", `apply tag hashtag
apply tag nestedtag: true
2011/01/27 Book Store
  expenses:books  $20.00
  liabilities:mastercard
end apply tag
end apply tag
`},
		{"account directive", "account expenses:food\n"},
		{"account directive with comment", "account expenses:food ; my account\n"},
		{"account with subdirectives", `account expenses:food
  note some note
  alias food  ; this gets ignored
`},
		{"comodity directive", "commodity $\n"},
		{"comodity directive word", "commodity UAH\n"},
		{"comodity directive no space", "commodity $1000.00\n"},
		{"commodity quantity first", "commodity 1,000.00 UAH\n"},
		{"commodity quantity after", "commodity UAH 1,000.00\n"},
		{"commodity with subdirectives", `commodity UAH
  format 1 000.00 UAH
  note Божествена Гривня  ; this gets ignored
`},
		{"payee directive", `payee grocery store
payee 'grocery store 3'
payee "grocery store 2"
payee grocery store 1
`},
		{"transaction", "2024/01/01\n"},
		{"automated transaction", `= ^income
    (liabilities:tax)  *.33

= expenses:gifts
    budget:gifts  *-1
    assets:budget  *1

= income:salary
    budget:savings  (amount * 0.5)
`},
		{"transaction with payee", "2024-01-01 groceries\n"},
		{"transaction with digit payees", `2002/01/01 * 1a1a6305d06ce4b284dba0d267c23f69d70c20be
    af0628973ff35bd62ddb048fa41dd8d83c1c46fe       $474.31
    fc6f6f10f627ad1a5af9d488c98405a1498d019d

2002/03/01 * 9861ce541c17b11f627e71c26bf350b33141f62b
    0ecbb1b15e2cf3e515cc0f8533e5bb0fb2326728        $14.91
    fc6f6f10f627ad1a5af9d488c98405a1498d019d
`},
		{"account with spaces", `2026-05-11 testies
	expenses:account name  20.00
	assets:bank
`},
		{"transaction with multiword payee", "2024-01-01 opening balances\n"},
		{"transaction pending", "2024-01-01 ! groceries\n"},
		{"transaction clearerd", "2024-01-01 * groceries\n"},
		{"transaction with note", "2024-01-01 groceries | eating out\n"},
		{"transaction with comment", "2024-01-01 groceries ; note\n"},
		{"transaction with secondary date", "2024/01/01=2024/01/02 groceries\n"},
		{"transaction with costs", `2026-05-11 testies
	expenses:atm  20.00 UAH @ 1 USD
	assets:bank

2026-05-11 testies2
	expenses:atm  20.00 UAH @@ 1 USD
	assets:bank

2026-05-12 testies3
	expenses:atm  20.00 UAH @ 1 USD = 20.00 UAH @ 1 USD
	assets:bank

2015-01-03 money exchange office
    assets:cash  -20 EUR @ 7.53 HRK
    assets:cash  150.60 HRK
`},
		{"transaction with cost and assertion", `2026-05-11 testies
	expenses:atm  20.00 UAH @ 1 USD = 0 UAH
	assets:bank
`},
		{"transaction with posting", `2024-01-01 groceries
    expenses:food  $10.00
    assets:checking
`},
		{"transaction with unicode commodity symbols", `2024-01-01 groceries
    expenses:food  €10.00
    expenses:food  £5.00
    expenses:food  ₹700.00
    assets:checking
`},
		{"transaction with strange commodity symbols", `2024-01-01 groceries
2026-05-20
  asdf  123 $€£
  asdf2

2026-05-20
  asdf  123 bytes
  asdf2
`},
		{"special chars in description", `
2024/01/01 groceries | 1 + 2
2024/01/01 groceries | 1 * 2
2024/01/01 groceries | 1 ! 2
2024/01/01 groceries | 1 # 2
2024/01/01 groceries | 1 % 2
2024/01/01 groceries | 1 ^ 2
2024/01/01 groceries | 1 & 2
2024/01/01 groceries | 1 ( 2
2024/01/01 groceries | 1 ) 2
2024/01/01 groceries | 1 [ 2
2024/01/01 groceries | 1 ] 2
2024/01/01 groceries | 1 { 2
2024/01/01 groceries | 1 } 2
2026-06-07 * payee !one | something *important*
    expenses:food  40.00
    assets:cash
`},
		{"transaction with tabs", `2024-01-01 groceries
	expenses:food  $10.00
	assets:checking
`},
		{"transaction in ukrainian", `2024/03/02 Обід
  витрати:їжа  350 UAH
  активи:готівка
`},
		{"transaction with code", `2024-01-01 (123) groceries
    expenses:food  $10.00
    assets:checking
`},
		{"transaction with posting amounts", `2024.01.01 groceries
    expenses:food  10.00 UAH
    assets:checking  -10.00 UAH
`},
		{"transaction with spaced account name", `2022-01-01 opening balances
    assets                    21 = 21
    equity:opening/closing balances
`},
		{"transaction with inline comment", `2024/01/01 groceries
	Expenses:Good  $10.00 ; food
	Assets:Checking
`},
		{"transaction with header comment", `2024/01/01 groceries
	; header comment
	expenses:food  $10.00
	assets:checking
`},
		{"transaction with trailing indent", `
2013/1/1 * pay taxes
    expenses:personal:tax              $1250
    assets:bank:checking               $-1250

`},
		{"transaction with balance assertion", `2024/01/01 groceries
	expenses:food  $10.00 = $100.00
	assets:checking

2025/03/07 groceries2
	expenses:food  $10.00 == $100.00
	assets:checking  == $0.00

2025/04/08 groceries3
	expenses:food  $10.00
	assets:checking  = $0.00
`},
		{"transaction with virtual accounts", `2024/01/01 groceries
	(virtual:account)  1 PESO
	[something:else]   5 PESO
	something:else
`},
		{"virtual postings with statuses", `2024/01/01 test
  ! (assets:cash)  $10
  (income:gift)  $-10

2024/01/01 test
    (! assets:cash)  $10
    (income:gift)  $-10

2024/01/01 test
    ! (! assets:cash)  $10
    (* income:gift)  $-10
`},
		{"period transaction expressions", `
~ monthly
    expenses:rent          $2000
    assets:bank:checking

~ monthly from 2023-04-15 to 2023-06-16
    expenses:utilities          $400
    assets:bank:checking

~ every 2 months  in 2023, we will review
    expenses:utilities          $400
    assets:bank:checking

~ monthly  Next year blah blah
    expenses:food  $100
    assets:checking

~ monthly from 2018/6 ;In 2019 we will change this
    expenses:food  $100
    assets:checking
`},
		{"unexpected token", `@@@ garbage\n`},
		{"recovery after bad posting", `2024-01-01 groceries
    @@@invalid
    assets:checking

2024/01/02 salary
    income:salary  $1000
    assets:checking
`},
		{"illegal only", "@@@\n"},
		{"illegal at start", "@@@ garbage\n"},
		{"illegal in posting", `2024/01/01 groceries
    @@@invalid
    assets:checking
`},
		{"illegal between transactions", `2024/01/01 groceries
    expenses:food  $10
    assets:checking
@@@
2024/01/02 salary
    income:salary  $1000
    assets:checking
`},
		{"multiple bad postings", `2024/01/01 groceries
    expenses:food  $10
    @@@bad1
    @bad2
    assets:checking
`},
		{"three bad postings", `2024/01/01 groceries
    expenses:food  $10
    @@@bad1
    @bad2
    @bad3
    assets:checking
`},
		{"quoted payee names", `2024-01-01 "groceries store"
  expenses:food  $10
  assets:checking
`},
		{"digit group marks", `2024-01-01 groceries
  expenses:food  1 000.00 UAH
  expenses:supplies  1'000.00 USD
  assets:checking  -2_000.00 USD
`},
		{"directive prefixes", `!account expenses:food
@commodity USD
`},
		{"bad between good", `2024/01/01 groceries
    expenses:food  $10
    @@@bad
    assets:cash  $5
`},
		{"all postings bad", `2024/01/01 groceries
    @@@bad1
    @bad2
    @bad3
`},
		{"bad then next transaction", `2024/01/01 groceries
    expenses:food  $10
    @@@bad

2024/01/02 salary
    income:salary  $1000
    assets:checking
`},
		{"comment between bad postings", `2024/01/01 groceries
    expenses:food  $10
    @@@bad1
    ; a comment
    @bad2
    assets:checking
`},
		{"bad posting at end", `2024/01/01 groceries
    expenses:food  $10
    @@@bad

2024/01/02 salary
    income:salary  $1000
    assets:checking
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New("j", []byte(tt.inp))
			f := New(l).ParseJournal()
			golden.Assert(t, ast.Dump(f))
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
