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
			if anchor != nil && o.compareDate(txn.Date, *anchor) < 0 {
				finds = append(finds, Find{
					Code:     o.ID(),
					Severity: o.Severity(),
					Message:  fmt.Sprintf("transaction is out of chronological order (date %s before %s)", o.dateString(txn.Date), o.dateString(*anchor)),
					Span:     txn.Date.Span,
				})
				continue
			}
			anchor = &txn.Date
		}
	}
	return finds
}

// compareDate returns -1 if a < b, 0 if equal, 1 if a > b.
func (o OrderDate) compareDate(a, b ast.Date) int {
	if a.Year != b.Year {
		if a.Year < b.Year {
			return -1
		}
		return 1
	}
	if a.Month != b.Month {
		if a.Month < b.Month {
			return -1
		}
		return 1
	}
	if a.Day != b.Day {
		if a.Day < b.Day {
			return -1
		}
		return 1
	}
	return 0
}

func (o OrderDate) dateString(d ast.Date) string {
	return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
}
