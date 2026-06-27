package linter

import "olexsmir.xyz/clerk/journal/ast"

// MissingPayee flags transactions with missing payee.
type MissingPayee struct{}

func (MissingPayee) ID() RuleID         { return "missing-payee" }
func (MissingPayee) Severity() Severity { return SeverityWarning }
func (m *MissingPayee) CheckEntry(entry ast.Entry) []Find {
	txn, ok := entry.(*ast.Transaction)
	if !ok {
		return nil
	}
	if txn.Payee == nil {
		return []Find{{
			Code:     m.ID(),
			Severity: m.Severity(),
			Message:  "transaction has no payee",
			Span:     txn.Date.Span,
		}}
	}
	return nil
}
