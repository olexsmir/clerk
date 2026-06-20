package linter

import "olexsmir.xyz/clerk/journal/ast"

// MissingCommodity flags amounts with a missing commodity.
type MissingCommodity struct{}

func (MissingCommodity) ID() RuleID         { return "missing-commodity" }
func (MissingCommodity) Severity() Severity { return SeverityWarning }
func (m *MissingCommodity) CheckEntry(entry ast.Entry) []Find {
	var finds []Find
	switch e := entry.(type) {
	case *ast.Transaction:
		m.checkPostings(&finds, e.Postings)
	case *ast.PeriodicTransaction:
		m.checkPostings(&finds, e.Postings)
	case *ast.AutomatedTransaction:
		m.checkPostings(&finds, e.Postings)
	case *ast.ConversionDirective:
		m.check(&finds, e.From)
		m.check(&finds, e.To)
	case *ast.CommodityDirective:
		m.check(&finds, e.Format)
	case *ast.DefaultCommodityDirective:
		m.check(&finds, e.Amount)
	case *ast.MarketPriceDirective:
		m.check(&finds, e.Amount)
	}
	return finds
}

func (m *MissingCommodity) checkPostings(finds *[]Find, postings []*ast.Posting) {
	for _, posting := range postings {
		if posting.Amount == nil {
			continue
		}
		m.check(finds, *posting.Amount)
		if posting.Cost != nil {
			m.check(finds, posting.Cost.Amount)
		}
		if posting.Balance != nil {
			m.check(finds, posting.Balance.Amount)
			if posting.Balance.Cost != nil {
				m.check(finds, posting.Balance.Cost.Amount)
			}
		}
	}
}

func (m *MissingCommodity) check(finds *[]Find, am ast.Amount) {
	if am.Commodity == "" {
		*finds = append(*finds, Find{
			Code:     m.ID(),
			Severity: m.Severity(),
			Message:  "amount missing commodity",
			Span:     am.Span,
		})
	}
}
