package linter

import "olexsmir.xyz/clerk/journal/ast"

// OmittedPrecision flags amounts with insufficient decimal precision (<2 digits).
type OmittedPrecision struct{}

func (OmittedPrecision) ID() RuleID          { return "omitted-precision" }
func (OmittedPrecision) Severity() Severity  { return SeverityWarning }
func (OmittedPrecision) Description() string { return "amount has insufficient precision" }
func (o *OmittedPrecision) CheckEntry(entry ast.Entry) []Find {
	txn, ok := entry.(*ast.Transaction)
	if !ok || (txn.Postings != nil && len(txn.Postings) == 0) {
		return nil
	}

	var finds []Find
	for _, posting := range txn.Postings {
		if posting.Amount == nil {
			continue
		}
		if posting.Amount.QuantityFmt.Precision < 2 {
			finds = append(finds, Find{
				Code:     o.ID(),
				Severity: o.Severity(),
				Message:  o.Description(),
				Span:     posting.Amount.Span,
			})
		}
	}
	return finds
}
