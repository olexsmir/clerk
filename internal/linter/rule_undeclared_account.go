package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/semantic"
)

// UndeclaredAccount flags postings that reference an account not declared via `account` directive.
type UndeclaredAccount struct{}

func (UndeclaredAccount) ID() RuleID         { return "undeclared-account" }
func (UndeclaredAccount) Severity() Severity { return SeverityWarning }
func (r *UndeclaredAccount) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for name, info := range ctx.Accounts {
		if len(info.Directives) > 0 {
			continue
		}
		for _, usage := range info.Usages {
			finds = append(finds, Find{
				Code:     r.ID(),
				Severity: r.Severity(),
				Span:     usage.Posting.Account.Span,
				Message:  fmt.Sprintf("undeclared account: %s", name),
			})
		}
	}
	return finds
}
