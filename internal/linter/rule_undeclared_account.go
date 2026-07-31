package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// UndeclaredAccount flags postings that reference an account not declared via `account` directive.
type UndeclaredAccount struct{}

func (UndeclaredAccount) ID() RuleID         { return "undeclared-account" }
func (UndeclaredAccount) Severity() Severity { return SeverityWarning }
func (u *UndeclaredAccount) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Accounts {
		if len(info.Directives) > 0 {
			continue
		}
		for _, usage := range info.Usages {
			finds = append(finds, Find{
				Code:     u.ID(),
				Severity: u.Severity(),
				Span:     usage.Posting.Account.Span,
				Message:  fmt.Sprintf("undeclared account: %s", name),
			})
		}
	}
	return finds
}
