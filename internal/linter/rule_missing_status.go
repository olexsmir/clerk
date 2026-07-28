package linter

import (
	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

// MissingStatus flags transactions with missing status.
type MissingStatus struct{}

func (MissingStatus) ID() RuleID         { return "missing-status" }
func (MissingStatus) Severity() Severity { return SeverityWarning }
func (m *MissingStatus) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		if txn.Status.Value == ast.StatusNone {
			finds = append(finds, Find{
				Code:     m.ID(),
				Severity: m.Severity(),
				Message:  "transaction has no status",
				Span:     txn.Status.Span,
			})
		}
	}
	return finds
}
