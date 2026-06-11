package journal_test

import (
	"path/filepath"
	"testing"

	"olexsmir.xyz/clerk/journal"
)

type test struct {
	desc string
	err  bool // file has intentional syntax errors, parser should report them
}

var tests = map[string]test{
	"actual-1ktxns-100accts.journal":            {desc: "hledger: stress test: 1000 transactions, 100 accounts, number-only account names"},
	"actual-accounttypes.journal":               {desc: "hledger: account type annotations (type:A, type:L) via comments"}, // todo: tags are not supported yet
	"actual-alias.journal":                      {desc: "hledger: account alias directives for renaming"},
	"actual-borrowing.journal":                  {desc: "hledger: borrowing/lending example with liabilities"},
	"actual-business.journal":                   {desc: "hledger: simple business transactions with commodities"},
	"actual-goal-budget.journal":                {desc: "hledger: goal budget using periodic transactions"},
	"actual-i18n-en.journal":                    {desc: "hledger: internationalization with account types in English"},
	"actual-ledger-baseline-opt-lots-basis.dat": {err: true, desc: "ledger: G/S prefixes"}, // no lot support yet
	"actual-ledger-input-divzero.dat":           {desc: "ledger: fuzz corpus, designed to cause divide-by-zero"},
	"actual-ledger-input-parsing.dat":           {desc: "ledger: fuzz corpus, tests EOF without newline"},
	"actual-ledger-input-sample.dat":            {desc: "ledger: fuzz corpus, default commodity directive"},
	"actual-ledger-input-standard.dat":          {desc: "ledger: fuzz corpus, standard ledger format"},
	"actual-ledger-input-transfer.dat":          {desc: "ledger: fuzz corpus, byte quantity (non-monetary)"},
	"actual-ledger-input-wow.dat":               {err: true, desc: "ledger-cli: fuzz corpus, World of Warcraft currency (1G=100s)"}, // no lot support ye
	"actual-multicurrency.journal":              {desc: "hledger: multi-currency transactions with HRK/EUR"},
	"actual-personal.journal":                   {desc: "hledger: simple personal finance example"},
	"actual-quickstart.journal":                 {desc: "hledger: quickstart guide with commodity directive"},
	"actual-sample.journal":                     {desc: "hledger: comprehensive sample with account tree"},
	"actual-sample2.journal":                    {desc: "hledger: sample2 with balance assertions and account directives"},
	"actual-status.journal":                     {desc: "hledger: tests all transaction statuses (unmarked, pending, cleared)"},
	"actual-templates.journal":                  {desc: "hledger: entry template examples with comments"},
	"actual-unicode.journal":                    {desc: "hledger: unicode in descriptions, account names, currency"},
	"actual-vat.journal":                        {desc: "hledger: VAT tracking example"},
	"apply-tag-block.dat":                       {desc: "ledger: apply-tag block directive"},
	"automated-posting-rule.dat":                {desc: "ledger: automated posting rules (= /^Expenses/)"},
	"basic-ledger.dat":                          {desc: "ledger: basic income/expense transaction"},
	"basic.journal":                             {desc: "hledger: minimal setup with account/commodity directives"},
	"broken-double-at.journal":                  {err: true, desc: "synthetic: intentionally broken (@@) syntax"},
	"broken-rparen.journal":                     {err: true, desc: "synthetic: intentionally broken unmatched )"},
	"broken-unknown-directive.journal":          {err: true, desc: "synthetic: intentionally broken unknown directive (??)"},
	"code-note.dat":                             {desc: "ledger: transaction with code, payee, comments"},
	"commodity-space.dat":                       {desc: "ledger: commodity with space before amount ($ 10.00)"},
	"cost-balance-assertion.dat":                {desc: "ledger: cost notation and balance assertion (@ 1 USD = 20.00 UAH)"},
	"directives-supported.journal":              {desc: "mixed: tests account, commodity, include, alias directives"},
	"ext-hledger-i18n-no.journal":               {desc: "hledger i18n example: uppercase directive values currently mis-tokenized"},
	"ext-hledger-self-tracking-d.dat":           {desc: "hledger self-tracking example with date+time transaction headers"},
	"ext-hledger-status.journal":                {desc: "hledger status example: ! (virtual:posting)"},
	"ext-ledger-parsing.dat":                    {desc: "ledger parsing corpus: -$ amount form"},
	"header-comments.journal":                   {desc: "hledger: transaction with header comment"},
	"inclusive-balance-star.journal":            {desc: "hledger: inclusive balance with ==*"},
	"multicurrency-supported.journal":           {desc: "hledger: working multi-currency with EUR exchange"},
	"periodic-basic.journal":                    {desc: "hledger: periodic transaction (~ monthly)"},
	"secondary-date-note.journal":               {desc: "hledger: secondary date and transaction note"},
	"status-basic.journal":                      {desc: "hledger: transaction status (pending with !)"},
	"unicode-cjk-emoji.journal":                 {desc: "synthetic: unicode CJK and emoji in transaction descriptions"},
	"unicode-cyrillic.journal":                  {desc: "synthetic: unicode Cyrillic in descriptions and account names"},
	"unicode-mixed-languages.journal":           {desc: "synthetic: mixed latin/cyrillic/cjk in descriptions and account names"},
	"virtual-posting.dat":                       {desc: "ledger: virtual/balanced postings with [brackets]"},
}

func TestParserOnRealJournals(t *testing.T) {
	for tname, tt := range tests {
		t.Run(tname, func(t *testing.T) {
			loader := journal.NewLoader()
			pf, err := loader.Load(filepath.Join("testdata/journals", tname))
			if err != nil {
				t.Fatalf("load err: %v", err)
			}

			if tt.err {
				if len(pf.Errors)+len(pf.FileErrors) == 0 {
					t.Errorf("expected parse errors but got none")
				}
				return
			}

			for _, e := range pf.Errors {
				t.Errorf("parse error: %s", e.Message)
			}
			for _, e := range pf.FileErrors {
				t.Errorf("file error [%s]: %s", e.Path, e.Message)
			}
		})
	}
}
