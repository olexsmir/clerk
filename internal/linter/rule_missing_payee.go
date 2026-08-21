package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const MissingPayeeID RuleID = "missing-payee"

// MissingPayee flags transactions with missing payee.
type MissingPayee struct{}

func (MissingPayee) ID() RuleID { return MissingPayeeID }
func (m *MissingPayee) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		if txn.Payee == nil {
			finds = append(finds, Find{
				Code:    m.ID(),
				Message: "transaction has no payee",
				Span:    txn.Date.Span,
			})
		}
	}
	return finds
}
