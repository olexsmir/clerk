package linter

import (
	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

// MultipleOmittedAmounts flags entries where more than one posting has an ommited amount.
type MultipleOmittedAmounts struct{}

func (MultipleOmittedAmounts) ID() RuleID         { return "multiple-omitted-amounts" }
func (MultipleOmittedAmounts) Severity() Severity { return SeverityError }
func (m *MultipleOmittedAmounts) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		finds = append(finds, m.check(txn.Postings)...)
	}
	for _, ptx := range an.PeriodicTransactions {
		finds = append(finds, m.check(ptx.Postings)...)
	}
	for _, atx := range an.AutomatedTransactions {
		finds = append(finds, m.check(atx.Postings)...)
	}
	return finds
}

func (m *MultipleOmittedAmounts) check(postings []*ast.Posting) []Find {
	var finds []Find
	for _, p := range postings {
		if p.Amount == nil && p.Balance == nil {
			finds = append(finds, Find{
				Code:     m.ID(),
				Severity: m.Severity(),
				Message:  "more than one posting has omitted amount",
				Span:     p.Span,
			})
		}
	}
	if len(finds) > 1 {
		return finds
	}
	return nil
}
