package linter

import (
	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

const MissingCommodityID RuleID = "missing-commodity"

// MissingCommodity flags amounts with a missing commodity.
type MissingCommodity struct{}

func (MissingCommodity) ID() RuleID { return MissingCommodityID }
func (m *MissingCommodity) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find

	for _, txn := range an.Transactions {
		m.checkPostings(&finds, txn.Postings)
	}
	for _, ptx := range an.PeriodicTransactions {
		m.checkPostings(&finds, ptx.Postings)
	}
	for _, atx := range an.AutomatedTransactions {
		m.checkPostings(&finds, atx.Postings)
	}

	for _, d := range an.Directives {
		switch e := d.(type) {
		case *ast.ConversionDirective:
			m.check(&finds, e.From)
			m.check(&finds, e.To)
		case *ast.CommodityDirective:
			if e.FormatSub != nil && e.FormatSub.Amount.Commodity != "" {
				m.check(&finds, e.FormatSub.Amount)
			}
		case *ast.DefaultCommodityDirective:
			m.check(&finds, e.Amount)
		case *ast.MarketPriceDirective:
			m.check(&finds, e.Amount)
		}
	}

	return finds
}

func (m *MissingCommodity) checkPostings(finds *[]Find, postings []ast.Posting) {
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
			Code:    m.ID(),
			Message: "amount missing commodity",
			Span:    am.Span,
		})
	}
}
