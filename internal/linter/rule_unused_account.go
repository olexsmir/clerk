package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// UnusedAccount flags declared accounts that are not used.
type UnusedAccount struct{}

func (UnusedAccount) ID() RuleID         { return "unused-account" }
func (UnusedAccount) Severity() Severity { return SeverityWarning }
func (a *UnusedAccount) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Accounts {
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
