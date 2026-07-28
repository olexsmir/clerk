package linter

import "olexsmir.xyz/clerk/internal/analyzer"

// OmittedPrecision flags amounts with insufficient decimal precision (<2 digits).
type OmittedPrecision struct{}

func (OmittedPrecision) ID() RuleID         { return "omitted-precision" }
func (OmittedPrecision) Severity() Severity { return SeverityWarning }
func (o *OmittedPrecision) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		for _, posting := range txn.Postings {
			if posting.Amount == nil {
				continue
			}
			if posting.Amount.QuantityFmt.Precision < 2 {
				finds = append(finds, Find{
					Code:     o.ID(),
					Severity: o.Severity(),
					Message:  "amount has insufficient precision",
					Span:     posting.Amount.Span,
				})
			}
		}
	}
	return finds
}
