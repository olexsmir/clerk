package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/semantic"
)

// DuplicatedAccount flags account declarations that appear more than once.
type DuplicatedAccount struct{}

func (DuplicatedAccount) ID() RuleID         { return "duplicated-account" }
func (DuplicatedAccount) Severity() Severity { return SeverityWarning }
func (d *DuplicatedAccount) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for _, info := range ctx.Accounts {
		if len(info.Directives) <= 1 {
			continue
		}
		for _, ad := range info.Directives {
			finds = append(finds, Find{
				Code:     d.ID(),
				Severity: d.Severity(),
				Message:  fmt.Sprintf("duplicated account declaration: %s", ad.Account.String()),
				Span:     ad.Account.Span,
			})
		}
	}
	return finds
}
