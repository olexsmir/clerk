package linter

import "olexsmir.xyz/clerk/journal/ast"

// MissingStatus flags transactions with missing status.
type MissingStatus struct{}

func (MissingStatus) ID() RuleID          { return "missing-status" }
func (MissingStatus) Severity() Severity  { return SeverityWarning }
func (MissingStatus) Description() string { return "transaction has no status" }
func (m *MissingStatus) CheckEntry(entry ast.Entry) []Find {
	tnx, ok := entry.(*ast.Transaction)
	if !ok {
		return nil
	}

	if tnx.Status.Value == ast.StatusNone {
		return []Find{{
			Code:     m.ID(),
			Severity: m.Severity(),
			Message:  m.Description(),
			Span:     tnx.Status.Span,
		}}
	}
	return nil
}
