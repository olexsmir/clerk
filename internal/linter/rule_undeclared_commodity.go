package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const UndeclaredCommodityID RuleID = "undeclared-commodity"

// UndeclaredCommodity flags amounts that reference a commodity not declared via `commodity` directive.
type UndeclaredCommodity struct{}

func (UndeclaredCommodity) ID() RuleID { return UndeclaredCommodityID }
func (u *UndeclaredCommodity) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Commodities {
		if len(info.Directives) > 0 {
			continue
		}
		for _, usage := range info.Usages {
			finds = append(finds, Find{
				Code:    u.ID(),
				Span:    usage.Amount.Span,
				Message: "undeclared commodity: " + name,
			})
		}
	}
	return finds
}
