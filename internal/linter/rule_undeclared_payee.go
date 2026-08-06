package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// UndeclaredPayee flags transactions that reference a payee not declared via `payee` directive.
type UndeclaredPayee struct{}

func (UndeclaredPayee) ID() RuleID         { return "undeclared-payee" }
func (UndeclaredPayee) Severity() Severity { return SeverityWarning }
func (u *UndeclaredPayee) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Payees {
		if name == "" {
			continue
		}
		if len(info.Directives) > 0 {
			continue
		}
		for _, usage := range info.Usage {
			finds = append(finds, Find{
				Code:     u.ID(),
				Severity: u.Severity(),
				Span:     usage.Payee.Span,
				Message:  fmt.Sprintf("undeclared payee: %s", name),
			})
		}
	}
	return finds
}
