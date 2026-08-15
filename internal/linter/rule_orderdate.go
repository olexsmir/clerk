package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

// OrderDate checks that transactions are in chronological order by date.
type OrderDate struct{}

func (OrderDate) ID() RuleID         { return "orderdate" }
func (OrderDate) Severity() Severity { return SeverityWarning }
func (o *OrderDate) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	var anchor *ast.Date
	for _, pf := range an.Files {
		for _, entry := range pf.Ast.Entries {
			txn, ok := entry.(*ast.Transaction)
			if !ok {
				continue
			}
			if anchor != nil && txn.Date.Compare(*anchor) < 0 {
				finds = append(finds, Find{
					Code:     o.ID(),
					Severity: o.Severity(),
					Message:  fmt.Sprintf("transaction is out of chronological order (date %s before %s)", txn.Date, *anchor),
					Span:     txn.Date.Span,
				})
				continue
			}
			anchor = &txn.Date
		}
	}
	return finds
}
