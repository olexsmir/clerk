package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// DuplicatedTransaction flags idnetical transactions.
type DuplicatedTransaction struct{}

func (DuplicatedTransaction) ID() RuleID         { return "duplicated-transaction" }
func (DuplicatedTransaction) Severity() Severity { return SeverityWarning }
func (d *DuplicatedTransaction) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txs := range an.TransactionsByKey {
		if len(txs) <= 1 {
			continue
		}
		for _, tx := range txs[1:] {
			finds = append(finds, Find{
				Code:     d.ID(),
				Severity: d.Severity(),
				Message:  fmt.Sprintf("duplicate of transaction at line %d", txs[0].Span.Start.Line),
				Span:     tx.Span,
			})
		}
	}
	return finds
}
