package ast

import (
	"fmt"
	"strings"
)

func Dump(f *Journal) string {
	var b strings.Builder
	dumpJournal(&b, f)
	return b.String()
}

func dumpJournal(b *strings.Builder, f *Journal) {
	fmt.Fprintf(b, "Journal\n")
	for _, e := range f.Entries {
		dumpEntry(b, e, 1)
	}
	if len(f.Errors) > 0 {
		fmt.Fprintf(b, "  Errors\n")
		for _, err := range f.Errors {
			fmt.Fprintf(b, "    %s: %s\n", err.Span, err.Message)
		}
	}
}

func dumpEntry(b *strings.Builder, e Entry, depth int) {
	switch e := e.(type) {
	case *Transaction:
		dumpTransaction(b, e, depth)
	case *PeriodicTransaction:
		dumpPeriodicTransaction(b, e, depth)
	case *AutomatedTransaction:
		dumpAutomatedTransaction(b, e, depth)
	case *BlankLine:
		indent(b, depth)
		fmt.Fprintf(b, "BlankLine %s\n", e.Span)
	case *Comment:
		dumpComment(b, e, depth)
	case *AccountDirective:
		dumpAccountDirective(b, e, depth)
	case *CommodityDirective:
		dumpCommodityDirective(b, e, depth)
	case *IncludeDirective:
		indent(b, depth)
		fmt.Fprintf(b, "IncludeDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Path: %q\n", e.Path)
		dumpOptComment(b, e.Comment, depth+1)
	case *AliasDirective:
		indent(b, depth)
		fmt.Fprintf(b, "AliasDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "From: %q\n", e.From)
		indent(b, depth+1)
		fmt.Fprintf(b, "To: %q\n", e.To)
	case *PayeeDirective:
		indent(b, depth)
		fmt.Fprintf(b, "PayeeDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Name: %q\n", e.Name)
		dumpOptComment(b, e.Comment, depth+1)
	case *TagDirective:
		indent(b, depth)
		fmt.Fprintf(b, "TagDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Name: %q\n", e.Name)
		dumpOptComment(b, e.Comment, depth+1)
	case *YearDirective:
		indent(b, depth)
		fmt.Fprintf(b, "YearDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Year: %d\n", e.Year)
	case *DecimalMarkDirective:
		indent(b, depth)
		fmt.Fprintf(b, "DecimalMarkDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Mark: %q\n", string(e.Mark))
	case *MarketPriceDirective:
		dumpMarketPriceDirective(b, e, depth)
	case *DefaultCommodityDirective:
		indent(b, depth)
		fmt.Fprintf(b, "DefaultCommodityDirective %s\n", e.Span)
		dumpAmount(b, &e.Amount, depth+1)
	case *ApplyDirective:
		indent(b, depth)
		fmt.Fprintf(b, "ApplyDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Expr: %q\n", e.Expr)
		dumpOptComment(b, e.Comment, depth+1)
	case *EndDirective:
		indent(b, depth)
		fmt.Fprintf(b, "EndDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Expr: %q\n", e.Expr)
		dumpOptComment(b, e.Comment, depth+1)
	case *CommentBlockDirective:
		indent(b, depth)
		fmt.Fprintf(b, "CommentBlockDirective %s\n", e.Span)
		indent(b, depth+1)
		fmt.Fprintf(b, "Header: %q\n", e.Header)
		indent(b, depth+1)
		fmt.Fprintf(b, "Content: %q\n", e.Content)
		dumpOptComment(b, e.Comment, depth+1)
	case *IgnoredDirective:
		indent(b, depth)
		fmt.Fprintf(b, "IgnoredDirective %s\n", e.Span)
	default:
		indent(b, depth)
		fmt.Fprintf(b, "Unknown %T\n", e)
	}
}

func indent(b *strings.Builder, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
}

func dumpTransaction(b *strings.Builder, t *Transaction, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "Transaction %s\n", t.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Date: %s\n", dumpDate(t.Date))
	if t.SecondDate != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "SecondDate: %s\n", dumpDate(*t.SecondDate))
	}
	if t.Status != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "State: %q\n", t.Status.Value)
	}
	if t.Code != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Code: %q\n", *t.Code)
	}
	if t.Payee != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Payee: %q %s\n", t.Payee.Name, t.Payee.Span)
	}
	if t.Note != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Note: %q\n", *t.Note)
	}
	dumpOptComment(b, t.Comment, depth+1)
	if len(t.HeaderComments) > 0 {
		indent(b, depth+1)
		fmt.Fprintf(b, "HeaderComments %s\n", t.Span)
		for _, c := range t.HeaderComments {
			dumpComment(b, &c, depth+2)
		}
	}
	for _, p := range t.Postings {
		dumpPosting(b, p, depth+1)
	}
}

func dumpAutomatedTransaction(b *strings.Builder, t *AutomatedTransaction, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "AutomatedTransaction %s\n", t.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Expr: %q\n", t.Expr)
	dumpOptComment(b, t.Comment, depth+1)
	if len(t.HeaderComments) > 0 {
		indent(b, depth+1)
		fmt.Fprintf(b, "HeaderComments %s\n", t.Span)
		for _, c := range t.HeaderComments {
			dumpComment(b, c, depth+2)
		}
	}
	for _, p := range t.Postings {
		dumpPosting(b, p, depth+1)
	}
}

func dumpPeriodicTransaction(b *strings.Builder, t *PeriodicTransaction, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "PeriodicTransaction %s\n", t.Span)
	if t.Period != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Period: %q\n", t.Period.Raw)
		if t.Period.From != nil {
			indent(b, depth+1)
			fmt.Fprintf(b, "From: %s\n", dumpDate(*t.Period.From))
		}
		if t.Period.To != nil {
			indent(b, depth+1)
			fmt.Fprintf(b, "To: %s\n", dumpDate(*t.Period.To))
		}
	}
	if t.Status != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Status: %q\n", t.Status.Value)
	}
	if t.Code != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Code: %q\n", *t.Code)
	}
	if t.Description != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Description: %q\n", *t.Description)
	}
	if t.Note != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Note: %q\n", *t.Note)
	}
	dumpOptComment(b, t.Comment, depth+1)
	if len(t.HeaderComments) > 0 {
		indent(b, depth+1)
		fmt.Fprintf(b, "HeaderComments\n")
		for _, c := range t.HeaderComments {
			dumpComment(b, c, depth+2)
		}
	}
	for _, p := range t.Postings {
		dumpPosting(b, p, depth+1)
	}
}

func dumpPosting(b *strings.Builder, p *Posting, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "Posting %s\n", p.Span)
	if p.Type != PostingReal {
		indent(b, depth+1)
		fmt.Fprintf(b, "Type: %s\n", p.Type)
	}
	if p.Status != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Status: %q\n", p.Status.Value)
	}
	dumpAccount(b, p.Account, depth+1)
	if p.Amount != nil {
		dumpAmount(b, p.Amount, depth+1)
	} else {
		indent(b, depth+1)
		fmt.Fprintf(b, "Amount: <elided>\n")
	}
	if p.Cost != nil {
		dumpCost(b, p.Cost, depth+1)
	}
	if p.Balance != nil {
		dumpBalanceAssertion(b, p.Balance, depth+1)
	}
	dumpOptComment(b, p.Comment, depth+1)
	if len(p.Comments) > 0 {
		for _, c := range p.Comments {
			dumpComment(b, &c, depth+1)
		}
	}
}

func dumpAmount(b *strings.Builder, a *Amount, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "Amount %s\n", a.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Quantity: %s\n", a.Quantity.String())
	indent(b, depth+1)
	fmt.Fprintf(b, "Commodity: %q\n", a.Commodity)
	indent(b, depth+1)
	fmt.Fprintf(b, "CommodityPos: %s\n", a.CommodityPos)
	indent(b, depth+1)
	fmt.Fprintf(b, "HasSpace: %v\n", a.HasSpace)
	if a.IsExpr {
		indent(b, depth+1)
		fmt.Fprintf(b, "IsExpr: true\n")
	}
	if a.Expr != "" {
		indent(b, depth+1)
		fmt.Fprintf(b, "Expr: %q\n", a.Expr)
	}
	indent(b, depth+1)
	fmt.Fprintf(b, "Precision: %d\n", a.QuantityFmt.Precision)
	indent(b, depth+1)
	fmt.Fprintf(b, "Decimal: %q\n", string(a.QuantityFmt.Decimal))
	if a.QuantityFmt.Thousands != 0 {
		indent(b, depth+1)
		fmt.Fprintf(b, "Thousands: %q\n", string(a.QuantityFmt.Thousands))
	}
}

func dumpCost(b *strings.Builder, c *Cost, depth int) {
	indent(b, depth)
	if c.IsTotal {
		fmt.Fprintf(b, "Cost(total) %s\n", c.Span)
	} else {
		fmt.Fprintf(b, "Cost(unit) %s\n", c.Span)
	}
	dumpAmount(b, c.Amount, depth+1)
}

func dumpBalanceAssertion(b *strings.Builder, ba *BalanceAssertion, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "BalanceAssertion %s\n", ba.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "IsStrict: %v\n", ba.IsStrict)
	indent(b, depth+1)
	fmt.Fprintf(b, "IsInclusive: %v\n", ba.IsInclusive)
	dumpAmount(b, &ba.Amount, depth+1)
}

func dumpAccount(b *strings.Builder, a Account, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "Account %q %s\n", a.Name, a.Span)
}

func dumpAccountDirective(b *strings.Builder, a *AccountDirective, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "AccountDirective %s\n", a.Span)
	dumpAccount(b, a.Account, depth+1)
	dumpOptComment(b, a.Comment, depth+1)
}

func dumpCommodityDirective(b *strings.Builder, c *CommodityDirective, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "CommodityDirective %s\n", c.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Commodity: %q\n", c.Commodity)
	if c.Format != nil {
		dumpAmount(b, c.Format, depth+1)
	}
	dumpOptComment(b, c.Comment, depth+1)
}

func dumpMarketPriceDirective(b *strings.Builder, m *MarketPriceDirective, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "MarketPriceDirective %s\n", m.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Date: %s\n", dumpDate(m.DateTime.Date))
	if m.DateTime.Time != nil {
		indent(b, depth+1)
		fmt.Fprintf(b, "Time: %02d:%02d:%02d\n", m.DateTime.Time.Hour, m.DateTime.Time.Minute, m.DateTime.Time.Second)
	}
	indent(b, depth+1)
	fmt.Fprintf(b, "Commodity: %q\n", m.Commodity)
	dumpAmount(b, &m.Amount, depth+1)
}

func dumpComment(b *strings.Builder, c *Comment, depth int) {
	indent(b, depth)
	fmt.Fprintf(b, "Comment %s\n", c.Span)
	indent(b, depth+1)
	fmt.Fprintf(b, "Marker: %q\n", string(c.Marker))
	indent(b, depth+1)
	fmt.Fprintf(b, "Text: %q\n", c.Text)
}

func dumpOptComment(b *strings.Builder, c *Comment, depth int) {
	if c == nil {
		return
	}
	dumpComment(b, c, depth)
}

func dumpDate(d Date) string {
	return fmt.Sprintf("%04d%s%02d%s%02d", d.Year, string(d.Sep), d.Month, string(d.Sep), d.Day)
}
