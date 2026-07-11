package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/semantic"
)

// UnusedAccount flags declared accounts that are not used.
type UnusedAccount struct{}

func (UnusedAccount) ID() RuleID         { return "unused-account" }
func (UnusedAccount) Severity() Severity { return SeverityWarning }
func (a *UnusedAccount) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for name, info := range ctx.Accounts {
		if len(info.Directives) == 0 {
			continue
		}
		if len(info.Usages) > 0 {
			continue
		}
		for _, d := range info.Directives {
			finds = append(finds, Find{
				Code:     a.ID(),
				Severity: a.Severity(),
				Span:     d.Account.Span,
				Message:  fmt.Sprintf("unused account: %s", name),
			})
		}
	}
	return finds
}
