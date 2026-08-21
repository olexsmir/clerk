package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const UndeclaredAccountID RuleID = "undeclared-account"

// UndeclaredAccount flags postings that reference an account not declared via `account` directive.
type UndeclaredAccount struct{}

func (UndeclaredAccount) ID() RuleID { return UndeclaredAccountID }
func (u *UndeclaredAccount) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Accounts {
		if len(info.Directives) > 0 {
			continue
		}
		if _, aliased := an.AccountAliases[name]; aliased {
			continue
		}
		for _, usage := range info.Usages {
			finds = append(finds, Find{
				Code:    u.ID(),
				Span:    usage.Posting.Account.Span,
				Message: "undeclared account: " + name,
			})
		}
	}
	return finds
}
