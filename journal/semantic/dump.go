package semantic

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

func fprint(w io.Writer, ctx *Context) {
	// Files
	fmt.Fprintf(w, "files (%d):\n", len(ctx.Files))
	for i, pf := range ctx.Files {
		fmt.Fprintf(w, "  %d: %s\n", i, filepath.Base(pf.Path))
	}

	// Commodities
	csyms := make([]string, 0, len(ctx.Commodities))
	for sym := range ctx.Commodities {
		csyms = append(csyms, sym)
	}
	sort.Strings(csyms)

	fmt.Fprintf(w, "\ncommodities (%d):\n", len(ctx.Commodities))
	for _, sym := range csyms {
		info := ctx.Commodities[sym]
		fmt.Fprintf(w, "  %s\n", sym)
		fmt.Fprintf(w, "    directives: %d\n", len(info.Directives))
		for _, cd := range info.Directives {
			fmt.Fprintf(w, "      commodity %s\n", cd.Commodity)
		}
		fmt.Fprintf(w, "    usages: %d\n", len(info.Usages))
		for _, u := range info.Usages {
			fmt.Fprintf(w, "      file %d: %s (%q)\n", u.FileIndex, u.Amount.Commodity, u.Amount.Quantity.String())
		}
	}

	// Accounts
	anames := make([]string, 0, len(ctx.Accounts))
	for name := range ctx.Accounts {
		anames = append(anames, name)
	}
	sort.Strings(anames)

	fmt.Fprintf(w, "\naccounts (%d):\n", len(ctx.Accounts))
	for _, name := range anames {
		info := ctx.Accounts[name]
		fmt.Fprintf(w, "  %s\n", name)
		fmt.Fprintf(w, "    directives: %d\n", len(info.Directives))
		for _, d := range info.Directives {
			fmt.Fprintf(w, "      account %s\n", d.Account.String())
		}
		fmt.Fprintf(w, "    usages: %d\n", len(info.Usages))
		for _, u := range info.Usages {
			fmt.Fprintf(w, "      file %d: %s\n", u.FileIndex, u.Posting.Account.String())
		}
	}
}
