package analyzer

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

func TestAnalysis(t *testing.T) {
	tests := []string{
		"empty",
		"include",
		"journal",
		"typed-entries",
		"account-indexing",
		"account-usage",
		"commodity-usage",
		"payees",
		"periodic-automated",
		"directives",
		"cost-balance",
		"posting-types",
		"dates",
		"tags",
		"decimal-marks",
		"duplicate-transactions",
	}

	for _, tname := range tests {
		t.Run(tname, func(t *testing.T) {
			a := golden.Read(t, tname)
			fsys, err := a.FS()
			if err != nil {
				t.Fatal(err)
			}

			l := journal.NewLoader()
			if _, err := l.LoadFS(fsys, "in.journal"); err != nil {
				t.Fatalf("failed to load test journal: %v", err)
			}

			ctx := Build(l.Ordered())
			var b strings.Builder
			fprint(&b, ctx)
			golden.Assert(t, a, b.String())
		})
	}
}

func fprint(w io.Writer, a *Analysis) {
	fmt.Fprintf(w, "files (%d):\n", len(a.Files))
	for i, pf := range a.Files {
		fmt.Fprintf(w, "  %d: %s\n", i, filepath.Base(pf.Path))
	}

	// commodities
	csyms := make([]string, 0, len(a.Commodities))
	for sym := range a.Commodities {
		csyms = append(csyms, sym)
	}
	sort.Strings(csyms)

	fmt.Fprintf(w, "\ncommodities (%d):\n", len(a.Commodities))
	for _, sym := range csyms {
		info := a.Commodities[sym]
		fmt.Fprintf(w, "  %s\n", sym)
		fmt.Fprintf(w, "    directives: %d\n", len(info.Directives))
		fmt.Fprintf(w, "    used: %d\n", info.UsedCount)
		for _, cd := range info.Directives {
			fmt.Fprintf(w, "      commodity %s\n", cd.Commodity)
		}
		fmt.Fprintf(w, "    usages: %d\n", len(info.Usages))
		for _, u := range info.Usages {
			fmt.Fprintf(w, "      file %d: %s (%q)\n", u.FileIndex, u.Amount.Commodity, u.Amount.Quantity.String())
		}
		if info.LastUsed.Year != 0 {
			fmt.Fprintf(w, "    last-used: %d-%02d-%02d\n", info.LastUsed.Year, info.LastUsed.Month, info.LastUsed.Day)
		}
	}

	// accounts
	anames := make([]string, 0, len(a.Accounts))
	for name := range a.Accounts {
		anames = append(anames, name)
	}
	sort.Strings(anames)

	fmt.Fprintf(w, "\naccounts (%d):\n", len(a.Accounts))
	for _, name := range anames {
		info := a.Accounts[name]
		fmt.Fprintf(w, "  %s\n", name)
		fmt.Fprintf(w, "    directives: %d\n", len(info.Directives))
		fmt.Fprintf(w, "    used: %d\n", info.UsedCount)
		for _, d := range info.Directives {
			fmt.Fprintf(w, "      account %s\n", d.Account.String())
		}
		fmt.Fprintf(w, "    usages: %d\n", len(info.Usages))
		for _, u := range info.Usages {
			fmt.Fprintf(w, "      file %d: %s\n", u.FileIndex, u.Posting.Account.String())
		}
		if info.LastUsed.Year != 0 {
			fmt.Fprintf(w, "    last-used: %d-%02d-%02d\n", info.LastUsed.Year, info.LastUsed.Month, info.LastUsed.Day)
		}
	}

	// payees
	pnames := make([]string, 0, len(a.Payees))
	for name := range a.Payees {
		pnames = append(pnames, name)
	}
	sort.Strings(pnames)

	fmt.Fprintf(w, "\npayees (%d):\n", len(a.Payees))
	for _, name := range pnames {
		info := a.Payees[name]
		fmt.Fprintf(w, "  %s\n", name)
		fmt.Fprintf(w, "    directives: %d\n", len(info.Directives))
		fmt.Fprintf(w, "    used: %d\n", info.UsedCount)
		for _, d := range info.Directives {
			fmt.Fprintf(w, "      payee %s\n", d.Name)
		}
		fmt.Fprintf(w, "    usages: %d\n", len(info.Usage))
		for _, u := range info.Usage {
			fmt.Fprintf(w, "      file %d: %s\n", u.FileIndex, u.Payee.Name)
		}
		if info.LastUsed.Year != 0 {
			fmt.Fprintf(w, "    last-used: %d-%02d-%02d\n", info.LastUsed.Year, info.LastUsed.Month, info.LastUsed.Day)
		}
	}

	// prefix index
	prefixes := make([]string, 0, len(a.AccountsByPrefix))
	for p := range a.AccountsByPrefix {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)
	fmt.Fprintf(w, "\nprefixes (%d):\n", len(a.AccountsByPrefix))
	for _, p := range prefixes {
		names := a.AccountsByPrefix[p]
		sort.Strings(names)
		fmt.Fprintf(w, "  %s\n", p)
		for _, n := range names {
			fmt.Fprintf(w, "    %s\n", n)
		}
	}

	// typed entry counts
	fmt.Fprintf(w, "\ntransactions: %d\n", len(a.Transactions))
	fmt.Fprintf(w, "periodic transactions: %d\n", len(a.PeriodicTransactions))
	fmt.Fprintf(w, "automated transactions: %d\n", len(a.AutomatedTransactions))

	// directives
	fmt.Fprintf(w, "\ndirectives (%d):\n", len(a.Directives))
	for _, d := range a.Directives {
		switch d := d.(type) {
		case *ast.AccountDirective:
			fmt.Fprintf(w, "  account %s\n", d.Account.String())
		case *ast.CommodityDirective:
			fmt.Fprintf(w, "  commodity %s\n", d.Commodity)
		case *ast.PayeeDirective:
			fmt.Fprintf(w, "  payee %s\n", d.Name)
		case *ast.TagDirective:
			fmt.Fprintf(w, "  tag %s\n", d.Name)
		case *ast.IncludeDirective:
			fmt.Fprintf(w, "  include %s\n", d.Path)
		case *ast.AliasDirective:
			fmt.Fprintf(w, "  alias %s = %s\n", d.From.String(), d.To.String())
		case *ast.YearDirective:
			fmt.Fprintf(w, "  year %d\n", d.Year)
		case *ast.DecimalMarkDirective:
			fmt.Fprintf(w, "  decimal-mark %c\n", d.Mark)
		case *ast.DefaultCommodityDirective:
			fmt.Fprintf(w, "  D %s%s\n", d.Amount.Commodity, d.Amount.Quantity.String())
		case *ast.MarketPriceDirective:
			fmt.Fprintf(w, "  P %s %s\n", d.Commodity, d.Amount.Quantity.String())
		case *ast.ConversionDirective:
			fmt.Fprintf(w, "  C %s%s @@ %s%s\n", d.From.Commodity, d.From.Quantity.String(), d.To.Commodity, d.To.Quantity.String())
		case *ast.ApplyDirective:
			fmt.Fprintf(w, "  apply %s\n", d.Expr)
		case *ast.EndDirective:
			fmt.Fprintf(w, "  end %s\n", d.Expr)
		case *ast.CommentBlockDirective:
			fmt.Fprintf(w, "  comment\n")
		case *ast.IgnoredDirective:
			fmt.Fprintf(w, "  N %s\n", d.Text)
		}
	}

	// transaction duplicate keys
	keys := make([]string, 0, len(a.TransactionsByKey))
	for k := range a.TransactionsByKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(w, "\ntransaction keys (%d):\n", len(a.TransactionsByKey))
	for _, k := range keys {
		txs := a.TransactionsByKey[k]
		if len(txs) > 1 {
			fmt.Fprintf(w, "  %s (dup: %d)\n", k, len(txs))
		} else {
			fmt.Fprintf(w, "  %s\n", k)
		}
	}

	// payee templates
	payeeNames := make([]string, 0, len(a.PayeeTemplates))
	for name := range a.PayeeTemplates {
		payeeNames = append(payeeNames, name)
	}
	sort.Strings(payeeNames)
	fmt.Fprintf(w, "\npayee templates (%d):\n", len(a.PayeeTemplates))
	for _, name := range payeeNames {
		templates := a.PayeeTemplates[name]
		fmt.Fprintf(w, "  %s\n", name)
		for _, t := range templates {
			fmt.Fprintf(w, "    %s", t.Account)
			if t.Amount != "" {
				fmt.Fprintf(w, "  %s%s", t.Commodity, t.Amount)
			}
			if t.IsInferred {
				fmt.Fprintf(w, " (inferred)")
			}
			fmt.Fprintln(w)
		}
	}

	// dates
	fmt.Fprintf(w, "\ndates (%d):\n", len(a.Dates))
	for _, d := range a.Dates {
		fmt.Fprintf(w, "  %d-%02d-%02d\n", d.Year, d.Month, d.Day)
	}

	// tag names
	fmt.Fprintf(w, "\ntag names (%d):\n", len(a.TagNames))
	for _, t := range a.TagNames {
		fmt.Fprintf(w, "  %s\n", t)
	}

	// commodity decimal marks
	csyms2 := make([]string, 0, len(a.CommodityDecimalMarks))
	for sym := range a.CommodityDecimalMarks {
		csyms2 = append(csyms2, sym)
	}
	sort.Strings(csyms2)
	fmt.Fprintf(w, "\ncommodity decimal marks (%d):\n", len(a.CommodityDecimalMarks))
	for _, sym := range csyms2 {
		fmt.Fprintf(w, "  %s: %c\n", sym, a.CommodityDecimalMarks[sym])
	}
}
