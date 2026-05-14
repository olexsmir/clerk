package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olexsmir/ledger-tools/journal"
)

type test struct {
	Desc   string
	Broken bool
}

var testCases = map[string]test{
	"actual-1ktxns-100accts.journal":   {Desc: "hledger stress test: 1000 transactions, 100 accounts, number-only account names"},
	"actual-accounttypes.journal":      {Desc: "hledger: account type annotations (type:A, type:L) via comments"},
	"actual-alias.journal":             {Desc: "hledger: account alias directives for renaming"},
	"actual-borrowing.journal":         {Desc: "hledger: borrowing/lending example with liabilities"},
	"actual-business.journal":          {Desc: "hledger: simple business transactions with commodities"},
	"actual-goal-budget-1.journal":     {Desc: "hledger: goal budget using periodic transactions"},
	"actual-i18n-en.journal":           {Desc: "hledger: internationalization with account types in English"},
	"actual-ledger-input-divzero.dat":  {Desc: "ledger-cli: fuzz corpus, designed to cause divide-by-zero"},
	"actual-ledger-input-parsing.dat":  {Desc: "ledger-cli: fuzz corpus, tests EOF without newline"},
	"actual-ledger-input-sample.dat":   {Desc: "ledger-cli: fuzz corpus, default commodity directive"},
	"actual-ledger-input-standard.dat": {Desc: "ledger-cli: fuzz corpus, standard ledger format"},
	"actual-ledger-input-transfer.dat": {Desc: "ledger-cli: fuzz corpus, byte quantity (non-monetary)"},
	"actual-ledger-input-wow.dat":      {Broken: true, Desc: "ledger-cli: fuzz corpus, World of Warcraft currency (1G=100s)"},
	"actual-multicurrency.journal":     {Desc: "hledger: multi-currency transactions with HRK/EUR"},
	"actual-personal.journal":          {Desc: "hledger: simple personal finance example"},
	"actual-quickstart.journal":        {Desc: "hledger: quickstart guide with commodity directive"},
	"actual-sample.journal":            {Desc: "hledger: comprehensive sample with account tree"},
	"actual-sample2.journal":           {Desc: "hledger: sample2 with balance assertions and account directives"},
	"actual-status.journal":            {Desc: "hledger: tests all transaction statuses (unmarked, pending, cleared)"},
	"actual-templates.journal":         {Desc: "hledger: entry template examples with comments"},
	"actual-unicode.journal":           {Desc: "hledger: unicode in descriptions, account names, currency"},
	"actual-vat.journal":               {Desc: "hledger: VAT tracking example"},
	"apply-tag-block.dat":              {Desc: "ledger-cli: apply-tag block directive"},
	"automated-posting-rule.dat":       {Desc: "ledger-cli: automated posting rules (= /^Expenses/)"},
	"basic-ledger.dat":                 {Desc: "ledger-cli: basic income/expense transaction"},
	"basic.journal":                    {Desc: "hledger: minimal setup with account/commodity directives"},
	"broken-double-at.journal":         {Broken: true, Desc: "synthetic: intentionally broken (@@) syntax"},
	"broken-rparen.journal":            {Broken: true, Desc: "synthetic: intentionally broken unmatched )"},
	"broken-unknown-directive.journal": {Broken: true, Desc: "synthetic: intentionally broken unknown directive (??)"},
	"code-note.dat":                    {Desc: "ledger-cli: transaction with code, payee, comments"},
	"commodity-space.dat":              {Desc: "ledger-cli: commodity with space before amount ($ 10.00)"},
	"cost-balance-assertion.dat":       {Desc: "ledger-cli: cost notation and balance assertion (@ 1 USD = 20.00 UAH)"},
	"directives-supported.journal":     {Desc: "mixed: tests account, commodity, include, alias directives"},
	"ext-hledger-i18n-no.journal":      {Desc: "hledger i18n example: uppercase directive values currently mis-tokenized"},
	"ext-hledger-self-tracking-d.dat":  {Desc: "hledger self-tracking example with date+time transaction headers"},
	"ext-hledger-status.journal":       {Desc: "hledger status example: ! (virtual:posting)"},
	"ext-ledger-parsing.dat":           {Desc: "ledger parsing corpus: -$ amount form"},
	"header-comments.journal":          {Desc: "hledger: transaction with header comment"},
	"inclusive-balance-star.journal":   {Desc: "hledger: inclusive balance with ==*"},
	"multicurrency-supported.journal":  {Desc: "hledger: working multi-currency with EUR exchange"},
	"periodic-basic.journal":           {Desc: "hledger: periodic transaction (~ monthly)"},
	"secondary-date-note.journal":      {Desc: "hledger: secondary date and transaction note"},
	"status-basic.journal":             {Desc: "hledger: transaction status (pending with !)"},
	"unicode-cjk-emoji.journal":        {Desc: "synthetic: unicode CJK and emoji in transaction descriptions"},
	"unicode-cyrillic.journal":         {Desc: "synthetic: unicode Cyrillic in descriptions and account names"},
	"unicode-mixed-languages.journal":  {Desc: "synthetic: mixed latin/cyrillic/cjk in descriptions and account names"},
	"virtual-posting.dat":              {Desc: "ledger-cli: virtual/balanced postings with [brackets]"},
}

func main() {
	failures := 0
	for name, tc := range testCases {
		loader := journal.NewLoader()
		pf, err := loader.Load(filepath.Join("tests/journal", name))
		if err != nil {
			if tc.Broken {
				fmt.Printf("SKIP %s: %s\n", name, tc.Desc)
				continue
			}
			fmt.Printf("FAIL %s: %v\n", name, err)
			failures++
			continue
		}

		if len(pf.Errors)+len(pf.FileErrors) > 0 {
			if tc.Broken {
				fmt.Printf("SKIP %s: %s\n", name, tc.Desc)
				continue
			}

			fmt.Printf("FAIL %s: %d parse errors, %d file errors\n", name, len(pf.Errors), len(pf.FileErrors))
			for i, e := range pf.Errors {
				if i >= 5 {
					fmt.Printf("  ... and %d more parse errors\n", len(pf.Errors)-5)
					break
				}
				fmt.Printf("  %s\n", e.Message)
			}
			for i, e := range pf.FileErrors {
				if i >= 5 {
					fmt.Printf("  ... and %d more file errors\n", len(pf.FileErrors)-5)
					break
				}
				fmt.Printf("  [%s] %s\n", e.Path, e.Message)
			}
			failures++
		} else {
			fmt.Printf("PASS %s\n", name)
		}
	}

	fmt.Printf("\n%d files failed\n", failures)
	if failures > 0 {
		os.Exit(1)
	}
}
