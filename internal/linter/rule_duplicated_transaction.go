package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

const DuplicatedTransactionID = "duplicated-transaction"

// DuplicatedTransaction flags idnetical transactions.
type DuplicatedTransaction struct{}

func (DuplicatedTransaction) ID() RuleID { return DuplicatedTransactionID }
func (d *DuplicatedTransaction) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txs := range an.TransactionsByKey {
		if len(txs) <= 1 {
			continue
		}
		for _, tx := range txs[1:] {
			finds = append(finds, Find{
				Code:    d.ID(),
				Message: fmt.Sprintf("duplicate of transaction at line %d", txs[0].Span.Start.Line),
				Span:    tx.Span,
			})
		}
	}
	return finds
}
