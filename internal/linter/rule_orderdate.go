package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/ast"
)

// OrderDate checks that transactions are in chronological order by date.
type OrderDate struct{}

func (OrderDate) ID() RuleID         { return "orderdate" }
func (OrderDate) Severity() Severity { return SeverityWarning }
func (r *OrderDate) CheckJournal(j *ast.Journal) []Find {
	var finds []Find
	var anchor *ast.Date
	for _, entry := range j.Entries {
		txn, ok := entry.(*ast.Transaction)
		if !ok {
			continue
		}

		d := txn.Date
		if anchor != nil {
			cmp := compareDate(d, *anchor)
			if cmp < 0 {
				finds = append(finds, Find{
					Code:     r.ID(),
					Severity: r.Severity(),
					Message: fmt.Sprintf("transaction is out of chronological order (date %s before %s)",
						dateString(d), dateString(*anchor)),
					Span: d.Span,
				})
				continue
			}
		}

		anchor = &d
	}

	return finds
}

// compareDate returns -1 if a < b, 0 if equal, 1 if a > b.
func compareDate(a, b ast.Date) int {
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

func dateString(d ast.Date) string {
	return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
}
