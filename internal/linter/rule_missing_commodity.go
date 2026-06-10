package linter

import "olexsmir.xyz/clerk/journal/ast"

// MissingCommodity flags amounts with a missing commodity.
type MissingCommodity struct{}

func (MissingCommodity) ID() RuleID          { return "missing-commodity" }
func (MissingCommodity) Severity() Severity  { return SeverityWarning }
func (MissingCommodity) Description() string { return "amount missing commodity" }
func (m *MissingCommodity) CheckEntry(entry ast.Entry) []Find {
	txn, ok := entry.(*ast.Transaction)
	if !ok || (txn.Postings != nil && len(txn.Postings) == 0) {
		return nil
	}

	var finds []Find
	for _, posting := range txn.Postings {
		if posting.Amount == nil {
			continue
		}
		if posting.Amount.Commodity == "" {
			finds = append(finds, Find{
				Code:     m.ID(),
				Severity: m.Severity(),
				Message:  m.Description(),
				Span:     posting.Amount.Span,
			})
		}
	}
	return finds
}
