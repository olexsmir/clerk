package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const EmptyPostingsID RuleID = "empty-postings"

// EmptyPostings flags transactions that have no postings.
type EmptyPostings struct{}

func (EmptyPostings) ID() RuleID { return EmptyPostingsID }
func (e *EmptyPostings) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		if len(txn.Postings) == 0 {
			finds = append(finds, Find{
				Code:    e.ID(),
				Message: "transaction has no postings",
				Span:    txn.Span,
			})
		}
	}
	return finds
}
