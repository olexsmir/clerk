package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const UndeclaredPayeeID RuleID = "undeclared-payee"

// UndeclaredPayee flags transactions that reference a payee not declared via `payee` directive.
type UndeclaredPayee struct{}

func (UndeclaredPayee) ID() RuleID { return UndeclaredPayeeID }
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
				Code:    u.ID(),
				Span:    usage.Payee.Span,
				Message: "undeclared payee: " + name,
			})
		}
	}
	return finds
}
