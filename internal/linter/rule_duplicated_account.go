package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

const DuplicatedAccountID RuleID = "duplicated-account"

// DuplicatedAccount flags account declarations that appear more than once.
type DuplicatedAccount struct{}

func (DuplicatedAccount) ID() RuleID { return DuplicatedAccountID }
func (d *DuplicatedAccount) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, info := range an.Accounts {
		if len(info.Directives) <= 1 {
			continue
		}
		for _, ad := range info.Directives {
			finds = append(finds, Find{
				Code:    d.ID(),
				Message: fmt.Sprintf("duplicated account declaration: %s", ad.Account.String()),
				Span:    ad.Account.Span,
			})
		}
	}
	return finds
}
