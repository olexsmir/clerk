package linter

import "olexsmir.xyz/clerk/internal/analyzer"

// EmptyPostings flags transactions that have no postings.
type EmptyPostings struct{}

func (EmptyPostings) ID() RuleID         { return "empty-postings" }
func (EmptyPostings) Severity() Severity { return SeverityError }
func (e *EmptyPostings) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		if len(txn.Postings) == 0 {
			finds = append(finds, Find{
				Code:     e.ID(),
				Severity: e.Severity(),
				Message:  "transaction has no postings",
				Span:     txn.Span,
			})
		}
	}
	return finds
}
