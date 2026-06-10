package linter

import "olexsmir.xyz/clerk/journal/ast"

// EmptyPostings flags transactions that have no postings.
type EmptyPostings struct{}

func (EmptyPostings) ID() RuleID          { return "empty-postings" }
func (EmptyPostings) Severity() Severity  { return SeverityError }
func (EmptyPostings) Description() string { return "transaction has no postings" }
func (e *EmptyPostings) CheckEntry(entry ast.Entry) []Find {
	txn, ok := entry.(*ast.Transaction)
	if !ok {
		return nil
	}
	if len(txn.Postings) == 0 {
		return []Find{{
			Code:     e.ID(),
			Severity: e.Severity(),
			Message:  e.Description(),
			Span:     txn.Span,
		}}
	}
	return nil
}
